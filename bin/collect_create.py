import statistics
import os
path = "."
durations = []
f = open("bin/server.8672.log")
counter = 0
for line in iter(f):
    if "creating" in line:
        for item in line.strip().split(","):
            if "used" in item:
                duration = float(item.split(" ")[2])
                durations.append(duration)
                counter += 1
    if counter == 3000:
        break
        
f.close()
print("mean is:", statistics.mean(durations))
print("var is:", statistics.variance(durations))
