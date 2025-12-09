import multiprocessing

import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc
import os

import numpy as np

import opfunu.cec_based.cec2022 as cec

NDIM=10

func = cec.F42022(ndim=NDIM)

LOW = func.lb[0]
HIGH = func.ub[0]
MUTATE_FACTOR = (HIGH-LOW)*0.1
MUTATION_RATE=0.3

SELECTION_RANDOMNESS=0.3

BASE_POPULATION_SIZE = 1000

class VolpeProblem(vp.VolpeContainerServicer):
    def __init__(self, fitness, gen_ind, mutate, crossover, select, encode, decode, encode_str, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.gen_ind = gen_ind
        self.fitness = fitness
        self.mutate = mutate
        self.crossover = crossover
        self.select = select
        self.encode = encode
        self.decode = decode
        self.encode_str = encode_str
        self.poplock = multiprocessing.Lock()

        self.cpu_count : int = os.cpu_count() if os.cpu_count() is not None else 1
        self.procPool = multiprocessing.Pool(self.cpu_count)

        poplnSplits = self.procPool.map(lambda n: return [ self.gen_ind() for _ in range(n) ],
                                        [ BASE_POPULATION_SIZE//self.cpu_count if i >= BASE_POPULATION_SIZE%self.cpu_count else BASE_POPULATION_SIZE//self.cpu_count+1 for i in range(self.cpu_count)]
                                        )
        self.popln = poplnSplits[0]
        for k in range(1, self.cpu_count):
            self.popln.extend(poplnSplits[k])

    def SayHello(self, request: pb.HelloRequest, context: grpc.ServicerContext):
        return pb.HelloReply(message="hello " + request.name)
    def InitFromSeed(self, request: pb.Seed, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        self.poplock.acquire()

        poplnSplits = self.procPool.map(lambda n: return [ self.gen_ind() for _ in range(n) ],
                                        [ BASE_POPULATION_SIZE//self.cpu_count if i >= BASE_POPULATION_SIZE%self.cpu_count else BASE_POPULATION_SIZE//self.cpu_count+1 for i in range(self.cpu_count)]
                                        )
        self.popln = poplnSplits[0]
        for k in range(1, self.cpu_count):
            self.popln.extend(poplnSplits[k])

        self.poplock.release()
        return pb.Reply(success=True)

    def InitFromSeedPopulation(self, request: pbc.Population, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        seedPop = [ self.decode(x.genotype) for x in request.members ]
        print("INCORPORATING POPLN OF LENGTH ", len(seedPop), " INTO ", len(self.popln))
        if self.popln == None:
            self.popln = []
        self.popln.extend(seedPop)
        self.popln = self.select(self.popln, BASE_POPULATION_SIZE)
        return pb.Reply(success=True)
    def GetBestPopulation(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popSorted = sorted(self.popln, key=self.fitness)
        resPop = popSorted[:request.size]
        return pbc.Population(members=[pbc.Individual(genotype=self.encode(x),
                                                                fitness=float(self.fitness(x)))
                                            for x in resPop])
    def GetResults(self, request: pb.PopulationSize, context):
        if self.popln is None:
            return pb.ResultPopulation(members=[])
        popSorted = sorted(self.popln, key=self.fitness)
        resPop = popSorted[:request.size]
        return pb.ResultPopulation(members=[pb.ResultIndividual(representation=self.encode_str(x),
                                                                fitness=float(self.fitness(x)))
                                            for x in resPop])
 
    def GetRandom(self, request: pb.PopulationSize, context):
        if self.popln is None:
            return pbc.Population(members=[], problemID="p1")
        popList = [ self.popln[i] for i in np.random.randint(0, len(self.popln), request.size) ]
        return pbc.Population(members=[pbc.Individual(genotype=self.encode(x),
                                                      fitness=float(self.fitness(x)))
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
            newinds = self.crossover(inds[0], inds[1])
            for i in range(len(newinds)):
                if np.random.random() < MUTATION_RATE:
                    newinds[i] = self.mutate(newinds[i])
            newpop.extend(newinds)
        self.popln.extend(newpop)
        self.popln = self.select(self.popln, ogLen)
        return pb.Reply(success=True)
