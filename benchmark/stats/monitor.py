import psutil
import csv
import time
import argparse
import sys

def log_system_utilization(filename, interval=5):
    header = ['CPU_Usage_Percent', 'Memory_Usage_GB']

    try:
        # Open in append mode
        with open(filename, mode='a', newline='') as file:
            writer = csv.writer(file)

            # Write header if file is new/empty
            if file.tell() == 0:
                writer.writerow(header)

            print(f"Logging to '{filename}' every {interval}s. Press Ctrl+C to stop.")

            while True:
                # cpu_percent(interval=1) captures usage over 1 second
                cpu = psutil.cpu_percent(interval=1)

                mem_bytes = psutil.virtual_memory().used
                mem_gb = round(mem_bytes / (1024 ** 3), 4)

                writer.writerow([cpu, mem_gb])
                file.flush()

                # Wait for the remainder of the 5-second cycle
                time.sleep(max(0, interval - 1))

    except PermissionError:
        print(f"Error: Could not write to {filename}. Is it open in another program?")
    except KeyboardInterrupt:
        print("\nMonitoring stopped.")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Log system CPU and Memory usage to a CSV.")

    # Add the filename argument
    parser.add_argument("filename", help="The name of the CSV file (e.g., stats.csv)")

    args = parser.parse_args()
    log_system_utilization(args.filename)