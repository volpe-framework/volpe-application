import greeter_test as gt

import numpy as np

serv = gt.VolpeGreeterServicer()

serv.InitFromSeed(gt.pb.Seed(seed=1), None)
print("initialized")
serv.AdjustPopulationSize(gt.pb.PopulationSize(size=200), None)
print("adjusted size")
for i in range(10):
    serv.RunForGenerations(None, None)
print("ran for gens")
res = serv.GetBestPopulation(gt.pb.PopulationSize(size=10), None)
serv.InitFromSeedPopulation(res, None)
res = serv.GetBestPopulation(gt.pb.PopulationSize(size=10), None)

for mem in res.members:
    print(mem.fitness)
