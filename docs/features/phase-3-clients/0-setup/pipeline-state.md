# Pipeline State: phase-3-clients
stage: done (Report)
updated: 2026-09-25T09:02:39Z
feature-base: d3de17e
stage-contract: cleared
contract-gaps: 3-design: 2-spec-interview/prd.md (тексты AC для ADR Driver — в бандле только заголовки AC); 1-research,2-spec-interview: docs/plan.md, prototype/** (источник требований без draft-spec)
profile: Бизнес-фича (tier M по D31; CM-1 MVP; UI-scope swagger:no / adaptive:min / themes:both — override из задачи)
spec-track: no (draft-spec отсутствует)
executor-note: fff не настроен в worktree (fff_preflight FAIL) — субагенты ищут штатным Grep/Read

## Agents (для SendMessage §3.9)
| агент | agentId | стадия запуска | фаза (Стадия 7) | жив? |
|---|---|---|---|---|
| research | a321e76a4fb9c3347 | 1 | — | да |
| spec-interview | a46fc6e0faf2970bc | 2 | — | да |
| design | a8b30fbd9ab56df43 | 3 | — | да |
| swimlane-designer | a9ba364fe510e6297 | 4 | — | да |
| design-reviewer | ab3cb63184682ec74 | 5 | — | да |
| mobile-api-reviewer | a6159cfd099b74fe4 | 5 | — | да |
| planner | a655912efedad7ec0 | 6 | — | да |
| backend-dev | a5a075c8cac671b1c | 7 | 01 | нет (фаза закрыта) |

## Counters
| цикл | итерация | max | при достижении max |
|---|---|---|---|
| qa-loop | 0 | 3 | AskUserQuestion (Стадия 10) |
| design-review-loop | 1 | 3 | AskUserQuestion |
| watch-gate | 1 | 3 | AskUserQuestion (TD-5) |
| artifact-lint:1-research | 1 | 2 | AskUserQuestion |

3.1-frontend-arch: N/A — Vue-фронтенда нет (D30: SSR html/template + vanilla JS); SSR-UI и E2E-сценарии спроектированы в 3-design/design-go-layers.md + design-tests.md
5.5-e2e-review: N/A — гард Vue (design-e2e-strategy.md не создаётся); E2E-сценарии — в design-tests.md, ревью на Стадии 5
5-design-review/frontend-arch: N/A — нет Vue (D30)
4-swimlane: повтор после fix-loop #1 не выполнялся — правки только в кодах ошибок/контракте, потоки swimlane не изменились
scaffold: skip — решение Tech Spec В-1 (вручную, 2 плоских пакета; отступление от D30 утверждено пользователем)
8-code-review/frontend-arch: N/A — нет Vue (D30)
