import requests
import json
import time
import csv
import sys
import argparse
import datetime
import re
import os # Added os import at the top level

# --- Configuration ---
DEFAULT_BASE_URL = "http://localhost:8000"
# LOG_INTERVAL_SECONDS and LAST_LOG_TIME removed as logging is now immediate

start_time = time.time()

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

def consume_sse(base_url, problem_id, output_filepath):
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
                            
                            # Log immediately to CSV
                            log_to_csv(output_filepath, problem_id, best_individual)
                            if time.time() >= 30:
                                print("time crossed 660, done")
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

if __name__ == "__main__":
    # os is now imported globally

    parser = argparse.ArgumentParser(
        description="Python client to consume Server-Sent Events (SSE) from VolPE and log the best fitness results to a CSV file."
    )
    parser.add_argument(
        "problem_id",
        type=str,
        help="The ID of the evolutionary problem to track (e.g., TSP-100)."
    )
    parser.add_argument(
        "--base-url",
        type=str,
        default=DEFAULT_BASE_URL,
        help=f"The base URL of the VolPE API (default: {DEFAULT_BASE_URL})."
    )
    parser.add_argument(
        "--output",
        type=str,
        default=None,
        help="The output CSV file path. Defaults to '{problem_id}_results.csv'."
    )

    args = parser.parse_args()

    # Determine output filename
    output_filename = args.output if args.output else f"{args.problem_id}_results.csv"

    print(f"--- VolPE SSE Client ---")
    print(f"Problem ID: {args.problem_id}")
    print(f"API URL: {args.base_url}")
    print(f"Log Path: {output_filename}")
    print(f"Logging Mode: Immediate (on receipt of data)")
    print("-" * 28)
    
    consume_sse(args.base_url, args.problem_id, output_filename)
