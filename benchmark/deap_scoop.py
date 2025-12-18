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
MAXTIME = 600

# Gaussian Mutation Parameters
MU = 0.0          # Mean of the Gaussian distribution (typically 0.0)
SIGMA = 5.0       # Standard deviation of the Gaussian distribution
INDPB = 1.0/DIMENSIONS # Probability for each attribute to be mutated

# Define the number of processes (workers) for clarity, though it's set at runtime by the `scoop` command
NUM_PROCESSES = 8 

# Get the CEC2022 F12 function object for 20 dimensions
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

# Evaluation function
def evaluate_cec(individual):
    # This function must be pickle-able (standard functions are fine)
    # and must be defined at the top level (global scope) for Scoop/multiprocessing
    return cec_f12.evaluate(np.array(individual)),

toolbox.register("evaluate", evaluate_cec)

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

# --- 3. Run the Optimization (Genetic Algorithm) ---

def main():
    print(f"Starting optimization of CEC2022 F12 (20D) using parallel DEAP/SCOOP...")

    start_time = time.time()
    
    pop = toolbox.population(n=POPULATION_SIZE)
    hof = tools.HallOfFame(1) 

    # Set up the statistics
    stats = tools.Statistics(lambda ind: ind.fitness.values)
    stats.register("avg", np.mean)
    stats.register("min", np.min)



    
    # Run the DEAP `eaSimple` algorithm. It will automatically use toolbox.map 
    # (which is set to scoop.futures.map) for parallel evaluation.
    try:
        with open('bench.csv', 'a') as f:
            writer = csv.writer(f)
            lastWrite = time.time()
            while True:
                pop, logbook = algorithms.eaSimple(pop, toolbox, cxpb=P_CROSSOVER, mutpb=P_MUTATION, 
                                                ngen=1, stats=stats, halloffame=hof, verbose=True)
                cur = time.time()
                if cur - lastWrite > 5:
                    best_ind=hof[0]
                    writer.writerow([cur-start_time, best_ind.fitness.values[0]])
                    lastWrite = cur
                    f.flush()
                if cur - start_time >= MAXTIME:
                    break
    except KeyboardInterrupt:
        print("Interrupted, exiting")

    # --- 4. Results ---
    
    print("\n--- Optimization Results ---")
    
    true_optimum = cec_f12.f_global
    print(f"True Global Optimum (F*): {true_optimum:.4f}")
    
    best_ind = hof[0]
    best_fitness = best_ind.fitness.values[0]
    
    print(f"Best Fitness Found: {best_fitness:.4f}")
    print(f"Difference (Best - True): {best_fitness - true_optimum:.4e}")
    print(f"Best Individual (First 5 dimensions): {best_ind[:5]}")
    
    # return pop, logbook, hof

if __name__ == "__main__":
    # SCOOP requires the entry point to be wrapped in the main function.
    # The actual execution happens when launching via the command line.
    main()
