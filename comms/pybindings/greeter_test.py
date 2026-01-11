from typing import Any, override
import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc
import concurrent.futures
import threading

from opfunu.cec_based.cec2022 import *

NDIM=20

func = F122022(ndim=NDIM)

LOW = func.lb[0]
HIGH = func.ub[0]
MUTATE_STD = 5.0
MUTATION_RATE = 0.2
INDPB = 1/NDIM
CXPROB = 0.9

BASE_POPULATION_SIZE = 50

import numpy as np


def fitness(x):
    return float(np.float32(func.evaluate(x)))

def mutate(x):
    mutated = x[0] + np.random.normal(size=NDIM, scale=MUTATE_STD) * \
            np.array([ 1 if np.random.random() < INDPB else 0 ])
    for i in range(NDIM):
        if mutated[i] < LOW:
            mutated[i] = LOW
        elif mutated[i] > HIGH:
            mutated[i] = HIGH
    if len(mutated) != NDIM:
        print("mutate")
    ind = np.astype(mutated, np.float32)
    return (ind, fitness(ind))
def varAnd(popln):
    ogLen = len(popln)
    popln = select(popln, ogLen)
    newpopln = []
    for i in range(0, ogLen, 2):
        if np.random.random() < CXPROB:
            i1 = i
            i2 = i+1
            n1, n2 = crossover(popln[i1], popln[i2])
            newpopln.append(n1)
            newpopln.append(n2)
        else:
            newpopln.append(popln[i])
            newpopln.append(popln[i+1])
    return mutate_popln(newpopln)

def crossover(x, y):
    rands = np.random.uniform(0, 1, size=NDIM)
    ind1 = np.array([x[0][i] if rands[i] == 0 else y[0][i] for i in range(NDIM)])
    ind2 = np.array([y[0][i] if rands[i] == 0 else x[0][i] for i in range(NDIM)])
    return (ind1, fitness(ind1)), (ind2, fitness(ind2))

def select(popln: list[tuple[np.ndarray, float]], newPop: int):
    newpopln = []
    while len(newpopln) < newPop:
        choices = [popln[choice(popln)] for _ in range(3)]
        selected = min(choices, key=lambda x: x[1])
        newpopln.append((selected[0].copy(), selected[1]))
    return newpopln

def choice(popln: list[Any]):
    l = len(popln)
    idx = np.random.randint(0, l)
    return idx

def gen_ind():
    ind = (np.random.random(size=NDIM) * (HIGH-LOW) + LOW).astype(np.float32)
    return (ind, fitness(ind))

def expand(popln: list[tuple[np.ndarray, float]], newPop: int):
    if len(popln) == 0:
        return [ gen_ind() for _ in range(newPop) ]
    if len(popln) >= newPop:
        return popln
    while len(popln) < newPop:
        x1 = popln[choice(popln)]
        x2 = popln[choice(popln)]
        y1, y2 = crossover(x1, x2)
        popln.append(y1)
        popln.append(y2)
    return popln

def get_random_list(popln: list[tuple[np.ndarray, float]], n: int):
    return [ popln[np.random.randint(len(popln))] for _ in range(n) ]

def mutate_popln(popln: list[tuple[np.ndarray, float]]):
    for i in range(len(popln)):
        if np.random.random() < MUTATION_RATE:
            popln[i] = mutate(popln[i])
    return popln

def popListTostring(popln: list[tuple[np.ndarray, float]]):
    indList : list[pb.ResultIndividual] = []
    for mem in popln:
        indList.append(
                pb.ResultIndividual(representation=np.array_str(mem[0]), 
                                    fitness=mem[1])
                )
    return pb.ResultPopulation(members=indList)

def bstringToPopln(popln: pbc.Population):
    popList = []
    for memb in popln.members:
        popList.append((np.frombuffer(memb.genotype, dtype=np.float32), memb.fitness))
    return popList

def adjustSize(popln: list[tuple[np.ndarray, float]], targetSize: int):
    print("ADJUSTSIZE called unexpectedly")
    if len(popln) < targetSize:
        return expand(popln, targetSize)
    else:
        return select(popln, targetSize)

def popListToBytes(popln: list[tuple[np.ndarray, float]]):
    indList : list[pbc.Individual] = []
    for mem in popln:
        indList.append(pbc.Individual(genotype=mem[0].astype(np.float32).tobytes(), fitness=mem[1]))
    return pbc.Population(members=indList, problemID="p1")

class VolpeGreeterServicer(vp.VolpeContainerServicer):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.popln : list[tuple[np.ndarray, float]] = [ gen_ind() for _ in range(BASE_POPULATION_SIZE)  ]
        self.poplock = threading.Lock()

    @override
    def SayHello(self, request: pb.HelloRequest, context: grpc.ServicerContext):
        return pb.HelloReply(message="hello " + request.name)
    @override
    def InitFromSeed(self, request: pb.Seed, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            self.popln = [ gen_ind() for _ in range(BASE_POPULATION_SIZE)  ]
            return pb.Reply(success=True)
    @override
    def InitFromSeedPopulation(self, request: pbc.Population, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            ogLen = len(self.popln)
            seedPop = bstringToPopln(request)
            if self.popln == None:
                self.popln = []
            self.popln.extend(seedPop)

            self.popln = select(self.popln, ogLen)

            return pb.Reply(success=True)
    @override
    def GetBestPopulation(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            if self.popln is None:
                return pbc.Population(members=[], problemID="p1")
            popSorted = sorted(self.popln, key=lambda x: x[1])
            return popListToBytes(popSorted[:request.size])
    @override
    def GetResults(self, request: pb.PopulationSize, context):
        with self.poplock:
            if self.popln is None:
                return pbc.Population(members=[], problemID="p1")
            popSorted = sorted(self.popln, key=lambda x: x[1])
            return popListTostring(popSorted[:request.size])
    @override
    def GetRandom(self, request: pb.PopulationSize, context):
        with self.poplock:
            if self.popln is None:
                return pbc.Population(members=[], problemID="p1")
            popList = get_random_list(self.popln, request.size)
            return popListToBytes(popList)
    @override
    def AdjustPopulationSize(self, request: pb.PopulationSize, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        # context.abort(grpc.StatusCode.CANCELLED, "AdjustPopulationSize not allowed")
        # targetSize = request.size
        # # TODO: adjust to targetSize
        # with self.poplock:
        #     self.popln = adjustSize(self.popln, BASE_POPULATION_SIZE)
        #     return pb.Reply(success=True)
    @override
    def RunForGenerations(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            ogLen = len(self.popln)
            popln = select(self.popln, ogLen)
            newpopln = [ ]
            for i in range(0, ogLen, 2):
                if np.random.random() < CXPROB:
                    i1 = i
                    i2 = i+1
                    n1, n2 = crossover(popln[i1], popln[i2])
                    newpopln.append(n1)
                    newpopln.append(n2)
                else:
                    newpopln.append(popln[i])
                    newpopln.append(popln[i+1])
            self.popln = mutate_popln(newpopln)
            self.popln = expand(self.popln, ogLen*2)
            self.popln = select(self.popln, ogLen)
        return pb.Reply(success=True)

if __name__=='__main__':
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=10))
    vp.add_VolpeContainerServicer_to_server(VolpeGreeterServicer(), server)
    server.add_insecure_port("0.0.0.0:8081")
    server.start()
    server.wait_for_termination()
