
from typing import Any
import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc
import concurrent.futures

import numpy as np

def fitness(x):
    return (1-x[0])**2 + 100*(x[1]-x[0])**2

def mutate(x):
    MUTATE_FACTOR = 0.08
    return np.astype(x + (np.random.random(size=2) * 2 - 1.0) * MUTATE_FACTOR, np.float32)

def crossover(x, y):
    alpha = np.random.random()
    return alpha*x + (1-alpha)*y

def reduce(popln: list[np.ndarray], newPop: int):
    if newPop >= len(popln):
        return popln
    sortedPop = sorted(popln, key=fitness)
    return sortedPop[:newPop]

def choice(popln: list[Any]):
    l = len(popln)
    idx = np.random.randint(0, l)
    return popln[idx]

def gen_ind():
    return (np.random.random(size=2) * 10.0 - 5.0).astype(np.float32)

def expand(popln: list[np.ndarray], newPop: int):
    if len(popln) == 0:
        return [ gen_ind() for _ in range(newPop) ]
    if len(popln) >= newPop:
        return popln
    while len(popln) < newPop:
        x1 = choice(popln)
        x2 = choice(popln)
        new_indi = crossover(x1, x2)
        popln.append(new_indi)
    return popln

def mutate_popln(popln: list[np.ndarray]):
    MUTATION_RATE=0.05
    for i in range(len(popln)):
        if np.random.random() < MUTATION_RATE:
            popln[i] = mutate(popln[i])
    return popln

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
        self.popln : list[np.ndarray] = [ gen_ind() for _ in range(100)  ]
    def SayHello(self, request: pb.HelloRequest, context: grpc.ServicerContext):
        return pb.HelloReply(message="hello " + request.name)
    def InitFromSeed(self, request: pb.Seed, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        self.popln = [ gen_ind() for _ in range(100)  ]
        return pb.Reply(success=True)

    def InitFromSeedPopulation(self, request: pbc.Population, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        seedPop = bstringToPopln(request)
        print("INCORPORATING POPLN OF LENGTH ", len(seedPop), " INTO ", len(self.popln))
        print(seedPop)
        if self.popln == None:
            self.popln = []
        self.popln.extend(seedPop)
        self.popln = adjustSize(self.popln, 100)
        print("BEST IS ", fitness(self.popln[0]), " vs ", min([fitness(x) for x in seedPop]))
        return pb.Reply(success=True)
    def GetBestPopulation(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popSorted = sorted(self.popln, key=fitness)
        return popListToBytes(popSorted[:request.size])
    def AdjustPopulationSize(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        targetSize = request.size
        self.popln = adjustSize(self.popln, targetSize)
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
