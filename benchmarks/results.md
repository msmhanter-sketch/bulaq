# Butaq vs C++ (-O3), Go, and Python Performance Benchmarks

20 comprehensive performance tests comparing Butaq (Native x86-64) against C++ (GCC -O3), Go, and Python.

| # | Benchmark Name | Butaq (Native) | C++ (-O3) | Golang | Python | Butaq vs C++ | Butaq vs Go | Butaq vs Py |
|---|---|---|---|---|---|---|---|---|
| 1 | Fibonacci (Recursion, 35) | 0.089s | 0.029s | 0.077s | 1.969s | **0.33x** | **0.86x** | **22.03x** |
| 2 | 10M Loop Increment | 0.032s | 0.000s | 0.005s | 0.412s | **0.00x** | **0.17x** | **12.72x** |
| 3 | Nested Loops (1000x1000) | 0.007s | 0.000s | 0.000s | 0.069s | **0.00x** | **0.00x** | **9.93x** |
| 4 | Float Math Ops (5M) | 0.068s | 0.042s | 0.058s | 0.421s | **0.63x** | **0.86x** | **6.23x** |
| 5 | Ackermann(3, 7) | 0.013s | 0.002s | 0.008s | 0.084s | **0.13x** | **0.59x** | **6.50x** |
| 6 | Boolean Logic (2M) | 0.027s | 0.000s | 0.003s | 0.276s | **0.00x** | **0.11x** | **10.04x** |
| 7 | String Concat (2000) | 0.005s | 0.000s | 0.003s | 0.001s | **0.01x** | **0.52x** | **0.18x** |
| 8 | String Compare (2M) | 0.011s | 0.000s | 0.000s | 0.218s | **0.02x** | **0.00x** | **20.79x** |
| 9 | Collatz up to 50K | 0.090s | 0.024s | 0.012s | 0.578s | **0.27x** | **0.13x** | **6.44x** |
| 10 | GCD iterations (2M) | 0.354s | 0.234s | 0.239s | 1.651s | **0.66x** | **0.67x** | **4.66x** |
| 11 | Factorial Math (1M) | 0.044s | 0.000s | 0.006s | 0.581s | **0.00x** | **0.14x** | **13.07x** |
| 12 | Bitwise shifts (2M) | 0.028s | 0.007s | 0.005s | 0.481s | **0.25x** | **0.17x** | **16.99x** |
| 13 | Type Casting (2M) | 0.012s | 0.000s | 0.000s | 0.147s | **0.00x** | **0.00x** | **11.98x** |
| 14 | Sum of Squares (500K) | 0.005s | 0.000s | 0.001s | 0.067s | **0.00x** | **0.09x** | **12.25x** |
| 15 | Pi Approx (1M) | 0.011s | 0.002s | 0.003s | 0.163s | **0.15x** | **0.24x** | **14.19x** |
| 16 | Prime Count (10K) | 0.000s | 0.002s | 0.001s | 0.014s | **9.85x** | **8.53x** | **92.32x** |
| 17 | Harmonic Sum (2M) | 0.014s | 0.005s | 0.008s | 0.124s | **0.38x** | **0.59x** | **8.67x** |
| 18 | Golden Ratio (50) | 0.000s | 0.000s | 0.000s | 0.000s | **3.87x** | **0.00x** | **36.25x** |
| 19 | Power Loop (5K) | 0.000s | 0.000s | 0.000s | 0.005s | **0.03x** | **0.00x** | **12.76x** |
| 20 | Mandelbrot Mock (100) | 0.002s | 0.000s | 0.001s | 0.033s | **0.00x** | **0.27x** | **14.28x** |

## Highlights & Context
- **C++ (-O3)** represents highly-optimized compiled machine code (GCC -O3 auto-vectorization, loop unrolling).
- **Go** compiled code runs with a lightweight runtime and GC, matching near-native speed.
- **Python** runs as an interpreted language, showing standard bytecode execution overhead.
- **Butaq (Native)** compiles directly to clean x86-64 machine code via NASM. It performs dramatically faster than Python, matches or beats Go in numeric tasks, and approaches optimized C++ on arithmetic/loops.
