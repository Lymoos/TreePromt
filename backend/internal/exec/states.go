// Package exec — домен Execution (AiCrew): задачи исполнения, воркеры, lease,
// state machine (ТЗ п. 7–8, docs/stage7-2-state-machine.md). Статусы меняет
// только этот пакет; модели и хосты лишь просят переход и сообщают факты.
package exec

import "time"

const (
	StatusQueued          = "QUEUED"
	StatusClaimed         = "CLAIMED"
	StatusInProgress      = "IN_PROGRESS"
	StatusVerifying       = "VERIFYING"
	StatusLLMReview       = "LLM_REVIEW"
	StatusMergeable       = "MERGEABLE"
	StatusMergeQueued     = "MERGE_QUEUED"
	StatusMerging         = "MERGING"
	StatusPostMergeVerify = "POST_MERGE_VERIFY"
	StatusMerged          = "MERGED"
	StatusDone            = "DONE"
	StatusRollbackQueued  = "ROLLBACK_QUEUED"
	StatusRollingBack     = "ROLLING_BACK"
	StatusRolledBack      = "ROLLED_BACK"
	StatusFailed          = "FAILED"
	StatusBlocked         = "BLOCKED"
	StatusReplan          = "REPLAN"
	StatusAwaitingHuman   = "AWAITING_HUMAN"
	StatusCancelling      = "CANCELLING"
	StatusCancelled       = "CANCELLED"
)

// Классы ошибок (ТЗ п. 8.4).
const (
	ClassBuild       = "BUILD_FAILURE"
	ClassTest        = "TEST_FAILURE"
	ClassTimeout     = "TIMEOUT"
	ClassNetwork     = "NETWORK_ERROR"
	ClassRateLimit   = "RATE_LIMIT"
	ClassAuth        = "AUTH_ERROR"
	ClassMerge       = "MERGE_CONFLICT"
	ClassIntegration = "INTEGRATION_FAILURE" // результат слияния не прошёл проверку (7.3)
	ClassEnvironment = "ENVIRONMENT_FAILURE"
	ClassQAReject    = "QA_REJECT"
	ClassAgent       = "AGENT_ERROR"
)

func ValidClass(c string) bool {
	switch c {
	case ClassBuild, ClassTest, ClassTimeout, ClassNetwork, ClassRateLimit, ClassAuth, ClassMerge, ClassIntegration,
		ClassEnvironment, ClassQAReject, ClassAgent:
		return true
	}
	return false
}

// workerTransitions — переходы, которые может запросить воркер, владеющий задачей.
var workerTransitions = map[string][]string{
	StatusClaimed:    {StatusInProgress, StatusFailed, StatusCancelled},
	StatusInProgress: {StatusVerifying, StatusFailed, StatusCancelled},
	StatusVerifying:  {StatusLLMReview, StatusFailed, StatusCancelled},
	StatusLLMReview:  {StatusMergeable, StatusFailed, StatusCancelled},
	// Очередь слияния (7.3): MERGEABLE → MERGE_QUEUED двигает пользователь или режим задачи.
	// MERGING → MERGED напрямую — восстановление, если integration уже содержит задачу.
	StatusMerging:         {StatusPostMergeVerify, StatusMerged, StatusFailed, StatusCancelled},
	StatusPostMergeVerify: {StatusMerged, StatusFailed, StatusCancelled},
	StatusMerged:          {StatusDone},
	StatusRollingBack:     {StatusRolledBack, StatusFailed},
	StatusCancelling:      {StatusCancelled, StatusFailed},
}

// leasedStatuses — пока задача в них, воркер обязан слать heartbeat.
var leasedStatuses = []string{StatusClaimed, StatusInProgress, StatusVerifying, StatusLLMReview, StatusCancelling,
	StatusMerging, StatusPostMergeVerify, StatusRollingBack}

// busyMergeStatuses — репозиторий занят очередью слияния: integration меняется строго по одной задаче.
var busyMergeStatuses = []string{StatusMerging, StatusPostMergeVerify, StatusRollingBack}

func WorkerCanMove(from, to string) bool {
	for _, s := range workerTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

func IsFinal(s string) bool { return s == StatusDone || s == StatusCancelled || s == StatusRolledBack }

// Политика повторов по классу ошибки (ТЗ п. 8.4).
type retryDecision struct {
	status       string
	delay        time.Duration
	spendAttempt bool
}

const (
	LeaseTTL             = 2 * time.Minute
	defaultRateLimitWait = 15 * time.Minute
	transientWait        = time.Minute
)

func decideRetry(class string, attempt, maxAttempts int, retryAfter *time.Duration) retryDecision {
	switch class {
	case ClassAuth:
		return retryDecision{status: StatusAwaitingHuman}
	case ClassRateLimit:
		d := defaultRateLimitWait
		if retryAfter != nil && *retryAfter > 0 {
			d = *retryAfter
		}
		// Лимит — не вина задачи: попытка не тратится.
		return retryDecision{status: StatusQueued, delay: d}
	case ClassNetwork, ClassTimeout, ClassEnvironment:
		if attempt >= maxAttempts {
			return retryDecision{status: StatusAwaitingHuman}
		}
		return retryDecision{status: StatusQueued, delay: transientWait, spendAttempt: true}
	default:
		// MERGE_CONFLICT и INTEGRATION_FAILURE — тоже сюда: задача переделывается поверх
		// свежего integration (решение 7.3, вопрос 3), попытка тратится.
		if attempt >= maxAttempts {
			return retryDecision{status: StatusAwaitingHuman}
		}
		return retryDecision{status: StatusQueued, spendAttempt: true}
	}
}
