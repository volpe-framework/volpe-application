
from typing import Any
import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc
import concurrent.futures

from opfunu.cec_based.cec2022 import *

NDIM=10

func = F42022(ndim=NDIM)

LOW = func.lb[0]
HIGH = func.ub[0]
MUTATE_FACTOR = (HIGH-LOW)*0.1
MUTATION_RATE=0.1

SELECTION_RANDOMNESS=0.2

BASE_POPULATION_SIZE = 100

import numpy as np

def fitness(x):
    return np.float32(func.evaluate(x))
def mutate(x):
    mutated = x + (np.random.random(size=NDIM) - 0.5) * MUTATE_FACTOR
    for i in range(NDIM):
        if mutated[i] < LOW:
            mutated[i] = LOW
        elif mutated[i] > HIGH:
            mutated[i] = HIGH
    return np.astype(mutated, np.float32)

def crossover(x, y):
    alpha = np.random.random()
    return alpha*x + (1-alpha)*y

def reduce(popln: list[np.ndarray], newPop: int):
    if newPop >= len(popln):
        return popln
    while len(popln) > newPop:
        c1 = choice(popln)
        c2 = choice(popln)
        invert = np.random.random() < SELECTION_RANDOMNESS
        if fitness(popln[c1]) > fitness(popln[c2]):
            if invert:
                popln.pop(c2)
            else:
                popln.pop(c1)
        elif fitness(popln[c2]) > fitness(popln[c1]):
            if invert:
                popln.pop(c1)
            else:
                popln.pop(c2)
    return popln

def choice(popln: list[Any]):
    l = len(popln)
    idx = np.random.randint(0, l)
    return idx

def gen_ind():
    return (np.random.random(size=NDIM) * (HIGH-LOW) + LOW).astype(np.float32)

def expand(popln: list[np.ndarray], newPop: int):
    if len(popln) == 0:
        return [ gen_ind() for _ in range(newPop) ]
    if len(popln) >= newPop:
        return popln
    while len(popln) < newPop:
        x1 = popln[choice(popln)]
        x2 = popln[choice(popln)]
        new_indi = crossover(x1, x2)
        popln.append(new_indi)
    return popln

def mutate_popln(popln: list[np.ndarray]):
    for i in range(len(popln)):
        if np.random.random() < MUTATION_RATE:
            popln[i] = mutate(popln[i])
    return popln

def popListTostring(popln: list[np.ndarray]):
    indList : list[pb.ResultIndividual] = []
    for mem in popln:
        indList.append(
                pb.ResultIndividual(representation=np.array_str(mem), 
                                    fitness=fitness(mem))
                )
    return pb.ResultPopulation(members=indList)

def bstringToPopln(popln: pbc.Population):
    popList = []
    for memb in popln.members:
        popList.append(np.frombuffer(memb.genotype, dtype=np.float32))
    return popList

def adjustSize(popln: list[np.ndarray], targetSize: int):
    if len(popln) < targetSize:
        return expand(popln, targetSize)
    else:
        return reduce(popln, targetSize)

def popListToBytes(popln: list[np.ndarray]):
    indList : list[pbc.Individual] = []
    for mem in popln:
        indList.append(pbc.Individual(genotype=mem.astype(np.float32).tobytes(), fitness=fitness(mem)))
    return pbc.Population(members=indList, problemID="p1")

class VolpeGreeterServicer(vp.VolpeContainerServicer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.popln : list[np.ndarray] = [ gen_ind() for _ in range(BASE_POPULATION_SIZE)  ]
    def SayHello(self, request: pb.HelloRequest, context: grpc.ServicerContext):
        return pb.HelloReply(message="hello " + request.name)
    def InitFromSeed(self, request: pb.Seed, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        self.popln = [ gen_ind() for _ in range(BASE_POPULATION_SIZE)  ]
        return pb.Reply(success=True)

    def InitFromSeedPopulation(self, request: pbc.Population, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        seedPop = bstringToPopln(request)
        print("INCORPORATING POPLN OF LENGTH ", len(seedPop), " INTO ", len(self.popln))
        print(seedPop)
        if self.popln == None:
            self.popln = []
        self.popln.extend(seedPop)
        self.popln = adjustSize(self.popln, BASE_POPULATION_SIZE)
        print("BEST IS ", fitness(self.popln[0]), " vs ", min([fitness(x) for x in seedPop]))
        return pb.Reply(success=True)
    def GetBestPopulation(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popSorted = sorted(self.popln, key=fitness)
        return popListToBytes(popSorted[:request.size])
    def GetResults(self, request: pb.PopulationSize, context):
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popSorted = sorted(self.popln, key=fitness)
        return popListTostring(popSorted[:request.size])

    def AdjustPopulationSize(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        targetSize = request.size
        # TODO: adjust to targetSize
        self.popln = adjustSize(self.popln, BASE_POPULATION_SIZE)
        return pb.Reply(success=True)
    def RunForGenerations(self, request, context):
        """Missing associated documentation comment in .proto file."""
        ogLen = len(self.popln)
        self.popln = mutate_popln(self.popln)
        self.popln = expand(self.popln, ogLen*2)
        self.popln = reduce(self.popln, ogLen)
        return pb.Reply(success=True)

if __name__=='__main__':
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=10))
    vp.add_VolpeContainerServicer_to_server(VolpeGreeterServicer(), server)
    server.add_insecure_port("0.0.0.0:8081")
    server.start()
    server.wait_for_termination()
