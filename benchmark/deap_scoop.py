import random
import numpy as np
import csv
import time
# Import the scoop map from deap.tools
from deap import base, creator, tools, algorithms
from opfunu.cec_based import cec2022
from scoop import futures # We will use scoop's map implementation

# --- 1. Define the Problem Parameters ---
DIMENSIONS = 20
POPULATION_SIZE = 400
P_CROSSOVER = 0.9
P_MUTATION = 0.2
MAXTIME = 175

# Gaussian Mutation Parameters
MU = 0.0          # Mean of the Gaussian distribution (typically 0.0)
SIGMA = 5.0       # Standard deviation of the Gaussian distribution
INDPB = 1.0/DIMENSIONS # Probability for each attribute to be mutated

# Define the number of processes (workers) for clarity, though it's set at runtime by the `scoop` command
NUM_PROCESSES = 8 

# Pre-instantiate CEC2022 function objects (accessible in worker processes)
cec_f7 = cec2022.F72022(ndim=DIMENSIONS)
cec_f11 = cec2022.F112022(ndim=DIMENSIONS)
cec_f12 = cec2022.F122022(ndim=DIMENSIONS)

# Define the bounds (lower and upper limits)
LOWER_BOUNDS = -100
UPPER_BOUNDS = 100

# --- Boundary Clamping Function ---
def clamp_individual(individual, low, up):
    """Clamps the individual's genes to the defined lower and upper bounds."""
    for i in range(len(individual)):
        individual[i] = max(low, min(up, individual[i]))
    return individual

# --- 2. DEAP Setup ---

# The CEC functions are MINIMIZATION problems (weights = -1.0)
creator.create("FitnessMin", base.Fitness, weights=(-1.0,))
creator.create("Individual", list, fitness=creator.FitnessMin)

# Initialize the Toolbox
toolbox = base.Toolbox()

# Attribute Generator
def get_random_float():
    return random.uniform(LOWER_BOUNDS, UPPER_BOUNDS)

# Structure Initializer
toolbox.register("attr_float", get_random_float)
toolbox.register("individual", tools.initRepeat, creator.Individual, 
                 toolbox.attr_float, n=DIMENSIONS)
toolbox.register("population", tools.initRepeat, list, toolbox.individual)

# Evaluation functions for each target
def evaluate_f7(individual):
    return cec_f7.evaluate(np.array(individual)),

def evaluate_f11(individual):
    return cec_f11.evaluate(np.array(individual)),

def evaluate_f12(individual):
    return cec_f12.evaluate(np.array(individual)),

# -----------------------------------------------------------------
# CRUCIAL CHANGE: Register scoop.futures.map as the parallel map function
toolbox.register("map", futures.map)
# -----------------------------------------------------------------

# Genetic Operators
# Crossover: Uniform Crossover
toolbox.register("mate", tools.cxUniform, indpb=0.5)

# Mutation: Gaussian Mutation with Boundary Clamping
def custom_gaussian_mutation(individual, mu, sigma, indpb, low, up):
    mutated_individual, = tools.mutGaussian(individual, mu, sigma, indpb)
    clamped_individual = clamp_individual(mutated_individual, low, up)
    return clamped_individual,

toolbox.register("mutate", custom_gaussian_mutation,
                 mu=MU,
                 sigma=SIGMA,
                 indpb=INDPB,
                 low=LOWER_BOUNDS,
                 up=UPPER_BOUNDS)

toolbox.register("select", tools.selTournament, tournsize=3)

def main():
    print("Starting sequential optimization for F7, F11, F12 (20D) using parallel DEAP/SCOOP...")

    # Iterate over requested functions in order
    for func_name in ["F7", "F11", "F12"]:
        print(f"\n=== Function {func_name} ===")

        # Run each function 10 times (individually)
        for run_no in range(1, 11):
            print(f"\n--- {func_name} Run {run_no}/10 ---")

            # Register the appropriate evaluation function and set current optimum
            if func_name == "F7":
                toolbox.register("evaluate", evaluate_f7)
                cec_current = cec_f7
            elif func_name == "F11":
                toolbox.register("evaluate", evaluate_f11)
                cec_current = cec_f11
            else:
                toolbox.register("evaluate", evaluate_f12)
                cec_current = cec_f12

            # Prepare output CSV per function/run
            output_filename = f"{func_name}_run{run_no}.csv"

            # Initialize population and tracking
            start_time = time.time()
            pop = toolbox.population(n=POPULATION_SIZE)
            hof = tools.HallOfFame(1)

            # Statistics for logging (optional verbose via DEAP)
            stats = tools.Statistics(lambda ind: ind.fitness.values)
            stats.register("avg", np.mean)
            stats.register("min", np.min)

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
                        pop, logbook = algorithms.eaSimple(
                            pop, toolbox,
                            cxpb=P_CROSSOVER, mutpb=P_MUTATION,
                            ngen=1, stats=stats, halloffame=hof, verbose=True
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
            true_optimum = cec_current.f_global
            best_ind = hof[0]
            best_fitness = best_ind.fitness.values[0]
            print(f"True Global Optimum (F*): {true_optimum:.4f}")
            print(f"Best Fitness Found: {best_fitness:.4f}")
            print(f"Difference (Best - True): {best_fitness - true_optimum:.4e}")
            print(f"Best Individual (First 5 dimensions): {best_ind[:5]}")
            

if __name__ == "__main__":
    # SCOOP requires the entry point to be wrapped in the main function.
    # The actual execution happens when launching via the command line.
    main()
