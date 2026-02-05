from array import array
import time
import csv

import tsplib95 as tsplib
from deap import base, creator, tools, algorithms
import random

import multiprocessing

# --- TSP Setup ---
problem = tsplib.load_problem('gil262.tsp')
NDIM = 262
BASE_POPULATION_SIZE = 100*12
LAMBDA_SIZE = BASE_POPULATION_SIZE*7
CXPROB = 0.5
MUTATION_RATE = 0.2
MAXTIME=33000
RUNS=5

# --- DEAP Configuration ---
creator.create("FitnessMin", base.Fitness, weights=(-1.0,))
# We define our Individual as a subclass of array.array
creator.create("Individual", array, fitness=creator.FitnessMin, typecode='i')

toolbox = base.Toolbox()

toolbox.register('indices', random.sample, range(NDIM), NDIM)
toolbox.register("individual", tools.initIterate, creator.Individual, toolbox.indices)
toolbox.register("population", tools.initRepeat, list, toolbox.individual)

def evaluate_tsp(individual):
    # Convert array back to list for tsplib if necessary
    return (problem.trace_tours([[i+1 for i in individual]])[0],)

toolbox.register("evaluate", evaluate_tsp)
toolbox.register("mate", tools.cxOrdered)
toolbox.register("mutate", tools.mutShuffleIndexes, indpb=0.05)
toolbox.register("select", tools.selTournament, tournsize=3)

def main():
    print("Starting sequential optimization for TSP262 using parallel DEAP/SCOOP...")

    # Iterate over requested functions in order
    for run_no in range(1, RUNS+1):
        print(f"\n--- TSP262 Run {run_no}/{RUNS} ---")
        # Prepare output CSV per function/run
        output_filename = f"TSP262_run{run_no}.csv"

        # Initialize population and tracking
        start_time = time.time()
        pop = toolbox.population(n=BASE_POPULATION_SIZE)
        hof = tools.HallOfFame(1)

        # Evaluate the initial population and push to HoF
        invalid_ind = [ind for ind in pop if not ind.fitness.valid]
        fitnesses = list(toolbox.map(toolbox.evaluate, invalid_ind))
        for ind, fit in zip(invalid_ind, fitnesses):
            ind.fitness.values = fit
        hof.update(pop)

        # Write pre-run best fitness once to the file (timestamp 0)
        try:
            with open(output_filename, 'a') as f:
                writer = csv.writer(f)
                writer.writerow([0.0, hof[0].fitness.values[0]])
                f.flush()
        except Exception as e:
            print(f"Failed to write pre-run fitness to {output_filename}: {e}")

        # Run the GA loop, logging best fitness every ~5 seconds
        try:
            with open(output_filename, 'a') as f:
                writer = csv.writer(f)
                last_write = time.time()
                while True:
                    pop, logbook = algorithms.eaMuCommaLambda(
                        pop, toolbox,
                        cxpb=CXPROB, mutpb=MUTATION_RATE,
                        lambda_=LAMBDA_SIZE,
                        mu=BASE_POPULATION_SIZE,
                        ngen=1, halloffame=hof, verbose=True
                    )
                    cur = time.time()
                    if cur - last_write > 5:
                        best_ind = hof[0]
                        writer.writerow([cur - start_time, best_ind.fitness.values[0]])
                        last_write = cur
                        f.flush()
                    if cur - start_time >= MAXTIME:
                        break
        except KeyboardInterrupt:
            print("Interrupted, exiting current run")

        # Report results for this run
        print("\n--- Run Results ---")
        best_ind = hof[0]
        best_fitness = best_ind.fitness.values[0]
        print(f"Best Fitness Found: {best_fitness:.4f}")
        print(f"Best Individual: {best_ind}")
            

if __name__ == "__main__":
    # SCOOP requires the entry point to be wrapped in the main function.
    # The actual execution happens when launching via the command line.
    pool = multiprocessing.Pool(processes=8)
    toolbox.register("map", pool.map)


    main()
 
