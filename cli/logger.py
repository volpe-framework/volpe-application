import requests
import json
import time
import csv
import sys
import argparse
import datetime
import re
import os # Added os import at the top level
import hashlib

# --- Configuration ---
DEFAULT_BASE_URL = "http://localhost:8000"
# LOG_INTERVAL_SECONDS and LAST_LOG_TIME removed as logging is now immediate

def compute_genotype_hash(genotype):
    """Computes the SHA256 hash of a genotype string."""
    genotype_str = str(genotype).strip()
    return hashlib.sha256(genotype_str.encode()).hexdigest()

def find_best_individual(population):
    """Finds the individual with the minimum fitness (assuming minimization)."""
    if not population:
        return None
    
    # Use min() with a lambda function to find the object with the lowest 'fitness'
    best = min(population, key=lambda x: x.get("fitness", float('inf')))
    return best

def log_to_csv(filepath, problem_id, best_individual):
    """Writes the best individual's data to the CSV file immediately upon receipt."""
    # Note: Removed time tracking logic as logging is now immediate.

    # Prepare data for logging
    timestamp_iso = time.time()
    # timestamp_iso = datetime.datetime.now().isoformat()
    fitness = best_individual.get("fitness")
    genotype = best_individual.get("genotype", "")

    # Clean up genotype string: normalize whitespace and remove surrounding quotes if present
    cleaned_genotype = genotype.strip().replace('\n', ' ').replace('\r', '').replace('\t', ' ')
    cleaned_genotype = re.sub(r'\s+', ' ', cleaned_genotype)

    # Prepare CSV row
    global start_time
    row = [timestamp_iso-start_time, fitness]

    try:
        # Check if file exists to determine if header is needed
        is_new_file = not os.path.exists(filepath)
        
        with open(filepath, 'a', newline='', encoding='utf-8') as f:
            writer = csv.writer(f)
            
            # Write header only if the file is newly created or empty
            # if is_new_file or os.stat(filepath).st_size == 0:
                # writer.writerow(["Timestamp (ISO)", "Problem ID", "Fitness", "Genotype"])
            
            # Write data row
            writer.writerow(row)
            
        print(f"[{datetime.datetime.now().strftime('%H:%M:%S')}] LOGGED: Best Fitness = {fitness:.4f}")

    except Exception as e:
        print(f"Error writing to CSV file: {e}", file=sys.stderr)

def log_to_summary_csv(filepath, run_no, best_individual, timestamp):
    """Writes to the summary CSV file with run_no, best_fitness, genotype_hash, genotype, and timestamp."""
    fitness = best_individual.get("fitness")
    genotype = best_individual.get("genotype", "")
    
    # Compute hash of genotype
    genotype_hash = compute_genotype_hash(genotype)
    
    # Round timestamp to int
    timestamp_int = int(round(timestamp))
    
    # Prepare CSV row
    row = [run_no, fitness, genotype_hash, genotype, timestamp_int]
    
    try:
        # Check if file exists to determine if header is needed
        is_new_file = not os.path.exists(filepath)
        
        with open(filepath, 'a', newline='', encoding='utf-8') as f:
            writer = csv.writer(f)
            
            # Write header only if the file is newly created
            if is_new_file:
                writer.writerow(["run_no", "best_fitness", "genotype_hash", "genotype", "timestamp"])
            
            # Write data row
            writer.writerow(row)
            
    except Exception as e:
        print(f"Error writing to summary CSV file: {e}", file=sys.stderr)

def consume_sse(base_url, problem_id, output_filepath, summary_filepath, run_no, stop_time=200):
    """Establishes SSE connection and processes incoming events."""
    print(f"Connecting to SSE stream for problem ID: {problem_id}")
    
    # Construct the full URL for the SSE endpoint
    url = f"{base_url}/problems/{problem_id}/results"
    
    # Use a streaming GET request with a long timeout
    try:
        response = requests.get(url, stream=True, timeout=None)
        response.raise_for_status() # Raise an exception for bad status codes (4xx or 5xx)

    except requests.exceptions.RequestException as e:
        print(f"Failed to connect to SSE endpoint at {url}: {e}", file=sys.stderr)
        return

    print("Connection successful. Awaiting data...")
    
    # --- SSE Message Processing Loop ---
    
    # Buffer for collecting partial events
    buffer = ""

    cur_start = time.time()
    
    # Track initial fitness to detect when EA starts improving
    initial_fitness = None
    last_fitness = None
    plateau_end_time = None
    initial_entry_logged = False
    
    for chunk in response.iter_content(chunk_size=1, decode_unicode=True):
        if not chunk:
            continue
        
        buffer += chunk
        done = False
        
        # Check for the SSE event delimiter (\n\n)
        if "\n\n" in buffer:
            events = buffer.split("\n\n")
            buffer = events.pop() # Keep the last incomplete part in the buffer
            
            for event in events:
                # An SSE event line starts with "data: "
                if event.startswith("data:"):
                    # Extract the JSON payload, removing "data: " and any newlines
                    json_data = event[len("data:"):].strip()
                    
                    try:
                        data = json.loads(json_data)
                        population = data.get("population", [])
                        
                        if population:
                            best_individual = find_best_individual(population)
                            best_fitness = best_individual.get("fitness")
                            
                            print(f"-> Received Fitness: {best_fitness:.4f}")
                            
                            # Track initial fitness
                            if initial_fitness is None:
                                initial_fitness = best_fitness
                                last_fitness = best_fitness
                            
                            # Check if fitness improved (end of plateau)
                            if plateau_end_time is None and best_fitness < initial_fitness:
                                plateau_end_time = time.time()
                                last_fitness = best_fitness
                                # Log initial entry when plateau ends (first improvement)
                                current_timestamp = time.time() - plateau_end_time
                                log_to_summary_csv(summary_filepath, run_no, best_individual, current_timestamp)
                                initial_entry_logged = True
                                print(f"-> Plateau ended, logged initial entry with fitness {best_fitness:.4f}")
                            
                            # Log subsequent improvements relative to plateau end time
                            elif plateau_end_time is not None and best_fitness < last_fitness:
                                last_fitness = best_fitness
                                current_timestamp = time.time() - plateau_end_time
                                log_to_summary_csv(summary_filepath, run_no, best_individual, current_timestamp)
                                print(f"-> Fitness improved to {best_fitness:.4f}, logged to summary")
                            
                            # Log immediately to CSV
                            log_to_csv(output_filepath, problem_id, best_individual)
                            if time.time() - cur_start >= stop_time:
                                print(f"{stop_time} seconds elapsed, stopping run")
                                done = True
                                break
                            
                        else:
                            print("-> Received empty population data.")

                    except json.JSONDecodeError:
                        print(f"Error decoding JSON: {json_data[:80]}...", file=sys.stderr)
                    except Exception as e:
                        print(f"Error processing data: {e}", file=sys.stderr)
            if done:
                break
    print("SSE stream closed by server.")

def create_problem(base_url, problem_id, image_path):
    """Creates a new problem via the API and returns the problem ID."""
    url = f"{base_url}/problems/{problem_id}"
    
    # Prepare metadata as JSON string
    metadata = {
        "problemID": problem_id,
        "memory": 0.5,
        "targetInstances": 12
    }
    
    try:
        # Open and send the image file with multipart form data
        with open(image_path, 'rb') as image_file:
            files = {
                'metadata': (None, json.dumps(metadata), 'application/json'),
                'image': ('grpc_test_img.tar', image_file, 'application/x-tar')
            }
            response = requests.post(url, files=files, timeout=30)
            response.raise_for_status()
            print(f"✓ Problem created with ID: {problem_id}")
            return problem_id
    except FileNotFoundError:
        print(f"✗ Image file not found: {image_path}", file=sys.stderr)
        sys.exit(1)
    except requests.exceptions.RequestException as e:
        print(f"✗ Failed to create problem: {e}", file=sys.stderr)
        sys.exit(1)

def start_problem(base_url, problem_id):
    """Starts/restarts the problem via the API."""
    url = f"{base_url}/problems/{problem_id}/start"
    
    try:
        response = requests.put(url, timeout=10)
        response.raise_for_status()
        print(f"✓ Problem {problem_id} started (Run initiated)")
        return True
    except requests.exceptions.RequestException as e:
        print(f"✗ Failed to start problem: {e}", file=sys.stderr)
        return False

def stop_problem(base_url, problem_id):
    """Stops/aborts the problem via the API."""
    url = f"{base_url}/problems/{problem_id}/abort"
    print("STOP URL:", url)
    
    try:
        response = requests.get(url, timeout=30)
        response.raise_for_status()
        print(f"✓ Problem {problem_id} stopped")
        return True
    except requests.exceptions.RequestException as e:
        print(f"✗ Failed to stop problem: {e}", file=sys.stderr)
        return False

if __name__ == "__main__":
    # os is now imported globally

    parser = argparse.ArgumentParser(
        description="Python client to create a problem and run it 10 times, logging results to CSV files."
    )
    parser.add_argument(
        "problem_name",
        type=str,
        help="The name for the problem (e.g., problem1)."
    )
    parser.add_argument(
        "image_path",
        type=str,
        nargs='?',
        default="../comms/pybindings/grpc_test_img.tar",
        help="Path to the container image tar file (default: ../comms/pybindings/grpc_test_img.tar)."
    )
    parser.add_argument(
        "--base-url",
        type=str,
        default=DEFAULT_BASE_URL,
        help=f"The base URL of the VolPE API (default: {DEFAULT_BASE_URL})."
    )
    parser.add_argument(
        "--runs",
        type=int,
        default=1,
        help="Number of times to run the problem (default: 1)."
    )
    parser.add_argument(
        "--stop-time",
        type=int,
        default=10000000,
        help="Number of seconds to run each problem before stopping (default: 10000000)."
    )

    args = parser.parse_args()

    print(f"--- VolPE SSE Client ---")
    print(f"Problem Name: {args.problem_name}")
    print(f"Image Path: {args.image_path}")
    print(f"API URL: {args.base_url}")
    print(f"Number of runs: {args.runs}")
    print("-" * 28)
    
    # Step 1: Create the problem once
    print("\n[Step 1] Creating problem...")
    problem_id = create_problem(args.base_url, args.problem_name, args.image_path)

    best_soln = ""
    
    # Step 2: Run the problem multiple times
    print(f"\n[Step 2] Running problem {args.runs} times...")
    for run_no in range(1, args.runs + 1):
        try:
            output_filename = f"{args.problem_name}_run{run_no}.csv"
            summary_filename = f"{args.problem_name}_summary.csv"
            
            print(f"\n--- Run {run_no}/{args.runs} ---")
            print(f"Log Path: {output_filename}")
            print(f"Summary Path: {summary_filename}")
            
            # Start the problem
            if not start_problem(args.base_url, problem_id):
                print(f"Skipping run {run_no} due to start failure.")
                continue
            
            # Consume SSE and log results
            global start_time
            start_time = time.time()
            consume_sse(args.base_url, problem_id, output_filename, summary_filename, run_no, args.stop_time)
        except KeyboardInterrupt:
            print("INTERRUPTED")
            
        print(f"✓ Run {run_no} completed")
            
        # Stop the problem after each run
        stop_problem(args.base_url, problem_id)
            
        # Small delay between runs
        if run_no < args.runs:
            time.sleep(15)
    
    print(f"\n✓ All {args.runs} runs completed!")
