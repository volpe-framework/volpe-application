from typing import Any
import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc

import concurrent.futures
import threading

from deap import base
from deap import creator
from deap import tools
from deap import algorithms

# import volpe_base as base

import opfunu.cec_based.cec2022 as cec

NDIM=10

func = cec.F42022(ndim=NDIM)

LOW = func.lb[0]
HIGH = func.ub[0]
MUTATE_FACTOR = (HIGH-LOW)*0.1
MUTATION_RATE=0.3
CROSSOVER_PROB=0.3

BASE_POPULATION_SIZE = 1000

import numpy as np

import scoop
from scoop import futures


creator.create("FitnessMin", base.Fitness, weights=(-1.0,))
creator.create("Individual", np.ndarray, fitness=creator.FitnessMin)
 

def checkBounds():
    def decorator(func):
        def wrapper(*args, **kargs):
            offspring = func(*args, **kargs)
            for child in offspring:
                for i in range(len(child)):
                    if child[i] > HIGH:
                        child[i] = HIGH
                    elif child[i] < LOW:
                        child[i] = LOW
            return offspring
        return wrapper
    return decorator

def cxTwoPointCopy(ind1, ind2):
    size=len(ind1)
    p1 = np.random.randint(1, size)
    p2 = np.random.randint(1, size-1)
    if p2 >= p1:
        p2 += 1
    else:
        p2, p1 = p1, p2
    ind1[p1:p2], ind2[p1:p2] = ind2[p1:p2].copy(), ind1[p1:p2].copy()
    return ind1, ind2

def fitness(x):
    return float(np.float32(func.evaluate(x))), 

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
    imax = 0
    for i in range(1, len(inds)):
        if inds[i][1] > inds[imax][1]:
            imax = i
    return inds[imax]
def select(popln: list[tuple[np.ndarray, float]], n: int):
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
def encode_str(x: np.ndarray):
    return np.array_str(x)

# def create_problem():
#     problem = base.VolpeProblem(fitness=fitness, gen_ind=gen_ind, mutate=mutate, crossover=crossover, select=select, encode=encode, decode=decode, encode_str=tostring)
# 
class VolpeProblem(vp.VolpeContainerServicer):
    def __init__(self, *args, **kwargs):
        '''
        Creates a new problem instance with the given functions.
        '''
        super().__init__(*args, **kwargs)

       
        self.toolbox = base.Toolbox()
        self.toolbox.register("attr_float", np.random.uniform, LOW, HIGH)
        self.toolbox.register("individual", tools.initRepeat, creator.Individual, self.toolbox.attr_float, n=NDIM)
        self.toolbox.register("population", tools.initRepeat, list, self.toolbox.individual)
        self.toolbox.register("map", futures.map)

        self.toolbox.register("mate", cxTwoPointCopy)
        self.toolbox.register("mutate", tools.mutGaussian, mu=0, sigma=5, indpb=MUTATION_RATE)
        self.toolbox.register("select", tools.selTournament, tournsize=3)
        self.toolbox.register("evaluate", fitness)
        self.toolbox.register("select_best", tools.selBest)

        self.toolbox.decorate("mate", checkBounds())
        self.toolbox.decorate("mutate", checkBounds())

        self.poplock = threading.Lock()

        self.popln = self.toolbox.population(n=BASE_POPULATION_SIZE)
        self.fitnesses = self.toolbox.map(self.toolbox.evaluate, self.popln)
        for ind, fit in zip(self.popln, self.fitnesses):
            ind.fitness.values = fit

    def SayHello(self, request: pb.HelloRequest, context: grpc.ServicerContext):
        return pb.HelloReply(message="hello " + request.name)
    def InitFromSeed(self, request: pb.Seed, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            self.popln = self.toolbox.population(n=BASE_POPULATION_SIZE)
            self.fitnesses = self.toolbox.map(self.toolbox.evaluate, self.popln)
            for ind, fit in zip(self.popln, self.fitnesses):
                ind.fitness.values = fit
            return pb.Reply(success=True)

    def InitFromSeedPopulation(self, request: pbc.Population, context: grpc.ServicerContext):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            seedPop = [ creator.Individual(decode(x.genotype)) for x in request.members ]
            seedFit = self.toolbox.map(self.toolbox.evaluate, seedPop)
            for ind, fit in zip(seedPop, seedFit):
                ind.fitness.setValues(fit)
            print("INCORPORATING POPLN OF LENGTH ", len(seedPop), " INTO ", len(self.popln))
            self.popln.extend(seedPop)
            self.popln = self.toolbox.select(self.popln, BASE_POPULATION_SIZE)
            return pb.Reply(success=True)

    def GetBestPopulation(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            if self.popln is None:
                return pbc.Population(members=[], problemID="p1")
            resPop = self.toolbox.select_best(self.popln, request.size)
            # TODO: calc fitness better
            return pbc.Population(members=[pbc.Individual(genotype=encode(x),
                                                                    fitness=float(x.fitness.values[0]))
                                                for x in resPop])
    def GetResults(self, request: pb.PopulationSize, context):
        with self.poplock:
            if self.popln is None:
                return pb.ResultPopulation(members=[])
            resPop = self.toolbox.select_best(self.popln, request.size)
            return pb.ResultPopulation(members=[pb.ResultIndividual(representation=encode_str(x),
                                                                    fitness=float(x[1]))
                                                for x in resPop])
 
    def GetRandom(self, request: pb.PopulationSize, context):
        with self.poplock:
            if self.popln is None:
                return pbc.Population(members=[], problemID="p1")
            popList = [ self.popln[i] for i in np.random.randint(0, len(self.popln), request.size) ]
            return pbc.Population(members=[pbc.Individual(genotype=encode(x),
                                                          fitness=float(x.fitness.values[0]))
                                           for x in popList])
    def AdjustPopulationSize(self, request: pb.PopulationSize, context):
        """Missing associated documentation comment in .proto file."""
        # targetSize = request.size
        # TODO: adjust to targetSize
        # self.popln = adjustSize(self.popln, BASE_POPULATION_SIZE)
        return pb.Reply(success=True)
    def RunForGenerations(self, request, context):
        """Missing associated documentation comment in .proto file."""
        with self.poplock:
            newpop = algorithms.varAnd(self.popln, self.toolbox, CROSSOVER_PROB, MUTATION_RATE)
            newFit = self.toolbox.map(self.toolbox.evaluate, newpop)
            ogLen = len(self.popln)
            for ind, fit in zip(newpop, newFit):
                ind.fitness.setValues(fit)
            self.popln = self.toolbox.select(newpop, ogLen)
            return pb.Reply(success=True)
    
if __name__=='__main__':
    problem = VolpeProblem()
    server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=10))
    vp.add_VolpeContainerServicer_to_server(problem, server)
    server.add_insecure_port("0.0.0.0:8081")
    server.start()
    server.wait_for_termination()
