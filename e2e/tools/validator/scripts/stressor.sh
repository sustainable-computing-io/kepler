#!/bin/bash

# warm up system by waiting for 40 seconds
# idle
stress-ng --cpu $(nproc) --cpu-load 0 --timeout 10s

# # Stress at 10% load for 20 seconds
# stress-ng --cpu $(nproc) --cpu-load 10 --timeout 20s
#
# # Stress at 25% load for 20 seconds
# stress-ng --cpu $(nproc) --cpu-load 25 --timeout 20s
#
# # Stress at 50% load for 20 seconds
# stress-ng --cpu $(nproc) --cpu-load 50 --timeout 20s
#
# Stress at 75% load for 10 seconds
stress-ng --cpu $(nproc) --cpu-load 75 --timeout 20s
#
# # Stress at 50% load for 20 seconds
# stress-ng --cpu $(nproc) --cpu-load 50 --timeout 20s
#
# # Stress at 25% load for 20 seconds
# stress-ng --cpu $(nproc) --cpu-load 25 --timeout 20s
#
# # Stress at 10% load for 20 seconds
# stress-ng --cpu $(nproc) --cpu-load 10 --timeout 20s

# cool off for 40 seconds
# idle
stress-ng --cpu $(nproc) --cpu-load 0 --timeout 10s

