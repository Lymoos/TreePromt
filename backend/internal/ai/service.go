package ai

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"prompttree/backend/internal/tree"
)

var (
	ErrDisabled     = errors.New("structuring is not configured on this server")
	ErrDailyLimit   = errors.New("daily structuring limit reached")
	modelTimeout    = 90 * time.Second
	workerCount     = 2
	queueBufferSize = 1000
)

// Service — очередь запросов на структурирование. Запрос асинхронный:
// клиент получает request_id сразу, результат приходит через синхронизацию.
type Service struct {
	tree       *tree.Service
	model      Model
	log        *slog.Logger
	dailyLimit int
	queue      chan uuid.UUID
	wg         sync.WaitGroup
	now        func() time.Time
}

func NewService(treeSvc *tree.Service, model Model, dailyLimit int, log *slog.Logger) *Service {
	return &Service{tree: treeSvc, model: model, log: log, dailyLimit: dailyLimit,
		queue: make(chan uuid.UUID, queueBufferSize), now: time.Now}
}

func (s *Service) Enabled() bool { return s.model != nil }

// Start запускает обработчики и возвращает в очередь запросы, прерванные перезапуском.
func (s *Service) Start(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-s.queue:
					s.process(ctx, id)
				}
			}
		}()
	}
	ids, err := s.tree.RequeueInterrupted(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.enqueue(ctx, id)
	}
	return nil
}

// Wait ждёт завершения обработчиков после отмены контекста Start.
func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) enqueue(ctx context.Context, id uuid.UUID) {
	select {
	case s.queue <- id:
	case <-ctx.Done():
	}
}

// Request ставит задачу в очередь. sourceRevision — ревизия исходника, которую видел клиент.
func (s *Service) Request(ctx context.Context, actor tree.Actor, nodeID uuid.UUID, sourceRevision int64) (uuid.UUID, error) {
	if !s.Enabled() {
		return uuid.Nil, ErrDisabled
	}
	if s.dailyLimit > 0 {
		n, err := s.tree.CountRecentRequests(ctx, actor.UserID, s.now().Add(-24*time.Hour))
		if err != nil {
			return uuid.Nil, err
		}
		if n >= s.dailyLimit {
			return uuid.Nil, ErrDailyLimit
		}
	}
	id := uuid.Must(uuid.NewV7())
	if err := s.tree.StartStructuring(ctx, actor, nodeID, sourceRevision, id, PromptVersion, s.model.Name()); err != nil {
		return uuid.Nil, err
	}
	s.enqueue(context.WithoutCancel(ctx), id)
	return id, nil
}

func (s *Service) process(ctx context.Context, id uuid.UUID) {
	job, err := s.tree.ClaimStructureRequest(ctx, id)
	if err != nil {
		s.log.Error("structure: claim", "request", id, "err", err)
		return
	}
	if job == nil {
		return // заменён более новым запросом
	}
	in := Input{Raw: job.Raw, Previous: job.Previous}
	mctx, cancel := context.WithTimeout(ctx, modelTimeout)
	gen, err := s.model.Generate(mctx, SystemPrompt(), BuildUserMessage(in))
	cancel()
	if err != nil {
		msg := "Не удалось получить ответ от ИИ. Попробуйте ещё раз позже."
		if errors.Is(err, ErrModelAuth) {
			msg = "Сервер не может обратиться к ИИ: ключ Gemini не подходит. Нужно проверить настройки сервера."
		}
		// В лог — причина без текста заметки и без ключа.
		s.log.Warn("structure: model failed", "request", id, "err", err)
		if err := s.tree.FailStructure(ctx, id, msg); err != nil {
			s.log.Error("structure: fail", "request", id, "err", err)
		}
		return
	}
	findings := Validate(in, gen)
	if err := s.tree.FinishStructure(ctx, id, gen, findings, PromptVersion); err != nil {
		s.log.Error("structure: finish", "request", id, "err", err)
	}
}
