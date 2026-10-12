/// Модели AiCrew для канбана (docs/stage7-6-security-audit.md, раздел 5). Только чтение того,
/// что отдаёт сервер; статусы меняет сервер.
library;

DateTime? _date(Object? v) => v is String ? DateTime.tryParse(v)?.toLocal() : null;
List<String> _strings(Object? v) => v is List ? v.whereType<String>().toList() : const [];

class ExecTask {
  const ExecTask({
    required this.id,
    required this.title,
    required this.status,
    required this.attempt,
    required this.maxAttempts,
    required this.mode,
    required this.updatedAt,
    this.failureClass,
    this.failureCode,
    this.mergeCommit,
    this.planId,
    this.executor = 'claude',
    this.touchesUI = false,
    this.skills = const [],
    this.approvalRequired = const [],
    this.reviewSummary,
    this.reviewer,
  });

  final String id, title, status, mode;
  final int attempt, maxAttempts;
  final DateTime updatedAt;
  final String? failureClass, failureCode, mergeCommit, planId, reviewSummary, reviewer;
  final String executor;
  final bool touchesUI;
  final List<String> skills, approvalRequired;

  factory ExecTask.fromJson(Map<String, dynamic> j) {
    final review = j['review'];
    return ExecTask(
      id: j['id'] as String,
      title: j['title'] as String? ?? '',
      status: j['status'] as String? ?? '',
      attempt: (j['attempt'] as num?)?.toInt() ?? 0,
      maxAttempts: (j['max_attempts'] as num?)?.toInt() ?? 0,
      mode: j['execution_mode'] as String? ?? 'manual',
      updatedAt: _date(j['updated_at']) ?? DateTime.now(),
      failureClass: j['failure_class'] as String?,
      failureCode: j['failure_code'] as String?,
      mergeCommit: j['merge_commit'] as String?,
      planId: j['plan_id'] as String?,
      executor: j['executor'] as String? ?? 'claude',
      touchesUI: j['touches_ui'] as bool? ?? false,
      skills: _strings(j['required_skills']),
      approvalRequired: _strings(j['approval_required']),
      reviewSummary: review is Map ? review['summary'] as String? : null,
      reviewer: review is Map ? review['reviewer'] as String? : null,
    );
  }
}

class PlanTaskView {
  const PlanTaskView({required this.title, required this.dependsOn});
  final String title;
  final List<int> dependsOn;
}

class ExecPlan {
  const ExecPlan({required this.id, required this.request, required this.status, required this.updatedAt,
      this.tasks = const [], this.rationale = '', this.error = ''});
  final String id, request, status, rationale, error;
  final DateTime updatedAt;
  final List<PlanTaskView> tasks;

  factory ExecPlan.fromJson(Map<String, dynamic> j) {
    final p = j['proposal'];
    final tasks = <PlanTaskView>[];
    var rationale = '';
    if (p is Map) {
      rationale = p['rationale'] as String? ?? '';
      for (final t in (p['tasks'] as List? ?? const [])) {
        if (t is Map) {
          tasks.add(PlanTaskView(
              title: t['title'] as String? ?? '',
              dependsOn: (t['depends_on'] as List? ?? const []).whereType<num>().map((e) => e.toInt()).toList()));
        }
      }
    }
    return ExecPlan(
      id: j['id'] as String,
      request: j['request'] as String? ?? '',
      status: j['status'] as String? ?? '',
      updatedAt: _date(j['updated_at']) ?? DateTime.now(),
      tasks: tasks,
      rationale: rationale,
      error: j['error'] as String? ?? '',
    );
  }
}

class Approval {
  const Approval({required this.id, required this.type, required this.status, this.taskId, this.planId, this.incidentId});
  final String id, type, status;
  final String? taskId, planId, incidentId;

  factory Approval.fromJson(Map<String, dynamic> j) => Approval(
        id: j['id'] as String,
        type: j['type'] as String? ?? '',
        status: j['status'] as String? ?? '',
        taskId: j['task_id'] as String?,
        planId: j['plan_id'] as String?,
        incidentId: j['incident_id'] as String?,
      );
}

class Incident {
  const Incident({required this.id, required this.kind, required this.reason, this.repositoryId});
  final String id, kind, reason;
  final String? repositoryId;

  factory Incident.fromJson(Map<String, dynamic> j) => Incident(
        id: j['id'] as String,
        kind: j['kind'] as String? ?? '',
        reason: j['reason'] as String? ?? '',
        repositoryId: j['repository_id'] as String?,
      );
}

class WorkerView {
  const WorkerView({required this.hostName, required this.kind, required this.model, required this.busy, required this.online});
  final String hostName, kind, model;
  final bool busy, online;

  factory WorkerView.fromJson(Map<String, dynamic> j) => WorkerView(
        hostName: j['host_name'] as String? ?? '',
        kind: j['kind'] as String? ?? '',
        model: j['model'] as String? ?? '',
        busy: j['status'] == 'busy',
        online: j['host_status'] == 'online',
      );
}

/// Всё, что показывает канбан.
class AiCrewBoard {
  const AiCrewBoard({this.tasks = const [], this.plans = const [], this.approvals = const [], this.incidents = const [],
      this.workers = const []});
  final List<ExecTask> tasks;
  final List<ExecPlan> plans;
  final List<Approval> approvals;
  final List<Incident> incidents;
  final List<WorkerView> workers;

  Approval? approvalFor({String? taskId, String? planId}) => approvals
      .where((a) => a.status == 'pending' && ((taskId != null && a.taskId == taskId) || (planId != null && a.planId == planId)))
      .firstOrNull;
}

class ExecRepository {
  const ExecRepository({required this.id, required this.projectId, required this.name, required this.localPath});
  final String id, projectId, name, localPath;

  factory ExecRepository.fromJson(Map<String, dynamic> j) => ExecRepository(
        id: j['id'] as String,
        projectId: j['project_id'] as String? ?? '',
        name: j['name'] as String? ?? '',
        localPath: j['local_path'] as String? ?? '',
      );
}
