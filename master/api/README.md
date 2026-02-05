# VolPE Master API

# EventStream

**Endpoint**: `GET <VOLPE_ENDPOINT>/eventStream`

Stream may include the following messages:

```
data: {"cpuCount":5,"mem":4.8127937,"type":"WorkerJoined","workerID":"worker_287214130873935974"}

data: {"islands":5,"mem":1,"problemID":"P1","type":"ProblemStarted"}

data: {"instances":4,"problemID":"P1","type":"SetWorkerInstances","workerID":"worker_287214130873935974"}

data: {"avgFitness":48951.4515625,"popSize":40,"problemID":"P1","type":"SentMasterPopulation","workerID":"worker_287214130873935974"}

data: {"avgFitness":48951.4515625,"popSize":40,"problemID":"P1","type":"ReceivedWorkerPopulation","workerID":"worker_287214130873935974"}

data: {"problemID":"P1","type":"ProblemStopped"}

data: {"type":"WorkerLeft","workerID":"worker_287214130873935974"}

data: { "type": "WorkerMetrics", "workerID": _, "cpuUtilPerc": _, "memUsageGB": _ }
```

