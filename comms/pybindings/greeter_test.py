from typing import Any
import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc
import concurrent.futures

import opfunu.cec_based.cec2022 as cec

NDIM=10

func = cec.F42022(ndim=NDIM)

LOW = func.lb[0]
HIGH = func.ub[0]
MUTATE_FACTOR = (HIGH-LOW)*0.1
MUTATION_RATE=0.3

SELECTION_RANDOMNESS=0.3

BASE_POPULATION_SIZE = 1000

import numpy as np

def gen_ind():
       return (np.random.random(size=NDIM) * (HIGH-LOW) + LOW).astype(np.float32)
def fitness(x):
    return np.float32(func.evaluate(x))

def mutate(x):
    mutated = x + (np.random.random(size=NDIM) - 0.5) * MUTATE_FACTOR
    for i in range(NDIM):
        if mutated[i] < LOW:
            mutated[i] = LOW
        elif mutated[i] > HIGH:
            mutated[i] = HIGH
    if len(mutated) != NDIM:
        print("mutate")
    return np.astype(mutated, np.float32)
def crossover(c1, c2):
    alpha = np.random.randint(0, 2, len(c1))
    p1 = c1*alpha + c2*(1-alpha)
    p2 = c2*alpha + c1*(1-alpha)
    if len(p1) != NDIM or len(p2) != NDIM:
        print("crossover")
    return [ p1, p2 ]
def tourney(popln, k=3):
    inds = [ popln[i] for i in np.random.randint(0, len(popln), k) ]
    inds.sort(key=fitness)
    if len(inds[0]) != NDIM:
        print(len(inds[0]))
        print("tourney")
    return inds[0]
def select(popln: list[np.ndarray], n: int):
    # Reduces the current population to n members
    if n >= len(popln):
        return popln
    new_pop = []
    while len(new_pop) < n:
        new_pop.append(tourney(popln))
    return new_pop
def encode(x: np.ndarray):
    return x.astype(np.float32).tobytes()
def decode(b: bytes):
    return np.frombuffer(b, dtype=np.float32)
def tostring(x: np.ndarray):
    return np.array_str(x)

class VolpeGreeterServicer(vp.VolpeContainerServicer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.popln = [ gen_ind() for _ in range(BASE_POPULATION_SIZE) ]
    def SayHello(self, request: pb.HelloRequest, context: grpc.ServicerContext):
        return pb.HelloReply(message="hello " + request.name)
    def InitFromSeed(self, request: pb.Seed, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        self.popln = [ gen_ind() for _ in range(BASE_POPULATION_SIZE)  ]
        return pb.Reply(success=True)

    def InitFromSeedPopulation(self, request: pbc.Population, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        seedPop = [ decode(x.genotype) for x in request.members ]
        print("INCORPORATING POPLN OF LENGTH ", len(seedPop), " INTO ", len(self.popln))
        if self.popln == None:
            self.popln = []
        self.popln.extend(seedPop)
        self.popln = select(self.popln, BASE_POPULATION_SIZE)
        return pb.Reply(success=True)
    def GetBestPopulation(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popSorted = sorted(self.popln, key=fitness)
        resPop = popSorted[:request.size]
        return pbc.Population(members=[pbc.Individual(genotype=encode(x),
                                                                fitness=float(fitness(x)))
                                            for x in resPop])
    def GetResults(self, request: pb.PopulationSize, context):
        if self.popln is None:
            return pb.ResultPopulation(members=[])
        popSorted = sorted(self.popln, key=fitness)
        resPop = popSorted[:request.size]
        return pb.ResultPopulation(members=[pb.ResultIndividual(representation=tostring(x),
                                                                fitness=float(fitness(x)))
                                            for x in resPop])
 
    def GetRandom(self, request: pb.PopulationSize, context):
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popList = [ self.popln[i] for i in np.random.randint(0, len(self.popln), request.size) ]
        return pbc.Population(members=[pbc.Individual(genotype=encode(x),
                                                      fitness=float(fitness(x)))
                                       for x in popList])
    def AdjustPopulationSize(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        # targetSize = request.size
        # TODO: adjust to targetSize
        # self.popln = adjustSize(self.popln, BASE_POPULATION_SIZE)
        return pb.Reply(success=True)
    def RunForGenerations(self, request, context):
        """Missing associated documentation comment in .proto file."""
        ogLen = len(self.popln)
        newpop = []
        while len(newpop) < len(self.popln):
            inds = [ self.popln[i] for i in np.random.randint(0, len(self.popln), 2) ]
            newinds = crossover(inds[0], inds[1])
            for i in range(len(newinds)):
                if np.random.random() < MUTATION_RATE:
                    newinds[i] = mutate(newinds[i])
            newpop.extend(newinds)
        self.popln.extend(newpop)
        self.popln = select(self.popln, ogLen)
        return pb.Reply(success=True)

if __name__=='__main__':
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=10))
    vp.add_VolpeContainerServicer_to_server(VolpeGreeterServicer(), server)
    server.add_insecure_port("0.0.0.0:8081")
    server.start()
    server.wait_for_termination()
