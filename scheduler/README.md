# scheduler

## master → scheduler

the master:

- creates a scheduler implementation (`PrelimScheduler` or `StaticScheduler`)
- registers workers via:
  - `AddWorker`
  - `RemoveWorker`
- registers problems via:
  - `AddProblem`
  - `RemoveProblem`
- forwards device metrics (optional)
- periodically calls:
  - `FillSchedule`

`FillSchedule` produces the desired `<problem@worker → instances>` mapping for the current cluster state.

## scheduler → master

the scheduler returns:

- a fully rebuilt `Schedule`
- target instance counts per `<problem, worker>` pair

the master is responsible for:

- diffing the schedule
- issuing scale up / scale down commands
- enforcing zero assignments for removed problems
