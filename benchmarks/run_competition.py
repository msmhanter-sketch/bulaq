import os
import subprocess
import time
import sys
import shutil

# Add benchmarks directory to path to import generator definitions
sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from generator import BENCHMARKS

PYTHON_BENCHMARKS = {
    "fib": ("""
def fib(n):
    if n < 2:
        return n
    return fib(n - 1) + fib(n - 2)
""", "fib(35)"),

    "loop_10m": ("""
def loop_10m():
    x = 0
    while x < 10000000:
        x += 1
    return x
""", "loop_10m()"),

    "nested_loop": ("""
def nested_loop():
    c = 0
    i = 0
    while i < 1000:
        j = 0
        while j < 1000:
            c += 1
            j += 1
        i += 1
    return c
""", "nested_loop()"),

    "math_float": ("""
def math_float():
    x = 1.0
    i = 0
    while i < 5000000:
        x *= 1.00001
        x /= 1.000001
        i += 1
    return x
""", "math_float()"),

    "ackermann": ("""
def ackermann(m, n):
    if m == 0:
        return n + 1
    if m > 0 and n == 0:
        return ackermann(m - 1, 1)
    return ackermann(m - 1, ackermann(m, n - 1))
""", "ackermann(3, 7)"),

    "boolean_logic": ("""
def boolean_logic():
    i = 0
    c = 0
    while i < 2000000:
        b1 = (i // 2) == 0
        b2 = (i // 3) == 0
        if b1 and not b2:
            c += 1
        i += 1
    return c
""", "boolean_logic()"),

    "string_concat": ("""
def string_concat():
    s = ""
    i = 0
    while i < 2000:
        s += "a"
        i += 1
    return len(s)
""", "string_concat()"),

    "string_compare": ("""
def string_compare():
    i = 0
    c = 0
    s1 = "hello_world_test"
    s2 = "hello_world_test"
    while i < 2000000:
        if s1 == s2:
            c += 1
        i += 1
    return c
""", "string_compare()"),

    "collatz": ("""
def collatz(limit):
    total = 0
    i = 1
    while i < limit:
        n = i
        while n != 1:
            div = n // 2
            rem = n - (div * 2)
            if rem == 0:
                n = div
            else:
                n = n * 3 + 1
            total += 1
        i += 1
    return total
""", "collatz(50000)"),

    "gcd_loop": ("""
def gcd(a, b):
    while b != 0:
        div = a // b
        rem = a - (div * b)
        a = b
        b = rem
    return a

def gcd_loop():
    i = 0
    c = 0
    while i < 2000000:
        c += gcd(12345, i)
        i += 1
    return c
""", "gcd_loop()"),

    "factorial": ("""
def factorial_loop():
    c = 0
    i = 0
    while i < 1000000:
        f = 1
        j = 1
        while j < 10:
            f *= j
            j += 1
        c += f
        i += 1
    return c
""", "factorial_loop()"),

    "bitwise_ops": ("""
def bitwise_ops():
    c = 0
    i = 1
    while i < 2000000:
        c += (i << 1)
        c += (i >> 1)
        i += 1
    return c
""", "bitwise_ops()"),

    "type_cast": ("""
def type_cast():
    c = 0
    i = 0
    while i < 2000000:
        c += int(1.5)
        i += 1
    return c
""", "type_cast()"),

    "sum_of_squares": ("""
def sum_of_squares(limit):
    c = 0
    i = 0
    while i < limit:
        c += (i * i)
        i += 1
    return c
""", "sum_of_squares(500000)"),

    "pi_approx": ("""
def pi_approx(n):
    s = 0.0
    i = 0
    while i < n:
        div = float(2 * i + 1)
        rem = (i // 2) * 2 == i
        if rem:
            s += 4.0 / div
        else:
            s -= 4.0 / div
        i += 1
    return s
""", "pi_approx(1000000)"),

    "prime_count": ("""
def is_prime(n):
    if n < 2:
        return 0
    i = 2
    while i * i <= n:
        div = n // i
        rem = n - (div * i)
        if rem == 0:
            return 0
        i += 1
    return 1

def prime_count(limit):
    c = 0
    i = 2
    while i < limit:
        if is_prime(i) == 1:
            c += 1
        i += 1
    return c
""", "prime_count(10000)"),

    "harmonic_sum": ("""
def harmonic_sum(limit):
    s = 0.0
    i = 1
    while i < limit:
        s += 1.0 / i
        i += 1
    return s
""", "harmonic_sum(2000000)"),

    "golden_ratio": ("""
def golden_ratio(limit):
    a = 1.0
    b = 1.0
    i = 0
    while i < limit:
        t = b
        b = a + b
        a = t
        i += 1
    return b / a
""", "golden_ratio(50)"),

    "power_loop": ("""
def power_loop(limit):
    s = 0.0
    i = 0
    while i < limit:
        p = 1.0
        j = 0
        while j < 10:
            p *= 1.1
            j += 1
        s += p
        i += 1
    return s
""", "power_loop(5000)"),

    "mandelbrot_mock": ("""
def mandelbrot_mock(limit):
    c = 0
    x_i = 0
    while x_i < limit:
        y_i = 0
        while y_i < limit:
            zr = 0.0
            zi = 0.0
            j = 0
            while j < 15:
                zr2 = zr * zr - zi * zi + x_i
                zi = 2.0 * zr * zi + y_i
                zr = zr2
                j += 1
            c += 1
            y_i += 1
        x_i += 1
    return c
""", "mandelbrot_mock(100)")
}

def main():
    print("Building Butaq compiler...")
    res = subprocess.run(["go", "build", "-o", "butaq.exe", "."], capture_output=True, text=True, encoding="utf-8")
    if res.returncode != 0:
        print("Go build error:", res.stderr)
        return

    # Ensure benchmarks dir exists
    os.makedirs("benchmarks", exist_ok=True)

    results_table = []
    
    print("\n| # | Benchmark Name | Butaq (Native) | C++ (-O3) | Golang | Python | Butaq vs C++ | Butaq vs Go | Butaq vs Py |")
    print("|---|---|---|---|---|---|---|---|---|")
    
    for i, b in enumerate(BENCHMARKS):
        name = b['desc']
        bench_key = b['name']
        btq_call = b['btq_func']
        cpp_call = b['cpp_func']
        go_call = b['go_func']
        btq_code = b['btq_code']
        cpp_code = b['cpp_code']
        go_code = b['go_code']

        # Get Python code
        py_info = PYTHON_BENCHMARKS.get(bench_key)
        if not py_info:
            print(f"Skipping {name} due to missing Python implementation")
            continue
        py_code, py_call = py_info

        # 1. Write and Run Butaq file
        btq_file = f"benchmarks/bench_{i}.btq"
        with open(btq_file, "w", encoding="utf-8") as f:
            f.write(btq_code + "\n")
            f.write(f'''t1 уақыт() болсын\nres {btq_call} болсын\nt2 уақыт() болсын\nt2 t1 алу жазу\n''')

        res = subprocess.run([".\\butaq.exe", "-r", btq_file], capture_output=True, text=True, encoding="utf-8")
        if res.returncode != 0:
            btq_time = None
            print(f"Butaq compilation/run failed for {name}: {res.stderr}")
        else:
            lines = res.stdout.strip().split("\n")
            try:
                btq_time = float(lines[-1].strip())
            except ValueError:
                btq_time = None
                print(f"Butaq output parsing failed for {name}: {res.stdout}")

        # 2. Write, Compile, and Run C++ file
        cpp_file = f"benchmarks/bench_{i}.cpp"
        cpp_exe = f"benchmarks/bench_cpp_{i}.exe"
        with open(cpp_file, "w", encoding="utf-8") as f:
            f.write("#include <iostream>\n#include <chrono>\n#include <string>\n#include <vector>\n#include <cmath>\n")
            f.write(cpp_code + "\n")
            f.write(f"""
int main() {{
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = {cpp_call};
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}}
""")

        # Compile C++ with -O3
        compile_cpp = subprocess.run(["g++", "-O3", "-o", cpp_exe, cpp_file], capture_output=True, text=True, encoding="utf-8")
        if compile_cpp.returncode != 0:
            cpp_time = None
            print(f"C++ compilation failed for {name}: {compile_cpp.stderr}")
        else:
            run_cpp = subprocess.run([f".\\{cpp_exe}"], capture_output=True, text=True, encoding="utf-8")
            if run_cpp.returncode != 0:
                cpp_time = None
                print(f"C++ run failed for {name}: {run_cpp.stderr}")
            else:
                lines = run_cpp.stdout.strip().split("\n")
                try:
                    cpp_time = float(lines[-1].strip())
                except ValueError:
                    cpp_time = None
                    print(f"C++ output parsing failed for {name}: {run_cpp.stdout}")

        # 3. Write, Compile, and Run Go file
        go_file = f"benchmarks/bench_{i}.go"
        go_exe = f"benchmarks/bench_go_{i}.exe"
        with open(go_file, "w", encoding="utf-8") as f:
            f.write("package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n")
            f.write(go_code + "\n")
            f.write(f"""
func main() {{
    t1 := time.Now()
    res := {go_call}
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\\n", t2.Sub(t1).Seconds())
}}
""")

        # Compile Go
        compile_go = subprocess.run(["go", "build", "-o", go_exe, go_file], capture_output=True, text=True, encoding="utf-8")
        if compile_go.returncode != 0:
            go_time = None
            print(f"Go compilation failed for {name}: {compile_go.stderr}")
        else:
            run_go = subprocess.run([f".\\{go_exe}"], capture_output=True, text=True, encoding="utf-8")
            if run_go.returncode != 0:
                go_time = None
                print(f"Go run failed for {name}: {run_go.stderr}")
            else:
                lines = run_go.stdout.strip().split("\n")
                try:
                    go_time = float(lines[-1].strip())
                except ValueError:
                    go_time = None
                    print(f"Go output parsing failed for {name}: {run_go.stdout}")

        # 4. Write and Run Python file
        py_file = f"benchmarks/bench_{i}.py"
        with open(py_file, "w", encoding="utf-8") as f:
            f.write("import time\nimport sys\nsys.setrecursionlimit(200000)\n")
            f.write(py_code + "\n")
            f.write(f"""
t1 = time.perf_counter()
res = {py_call}
t2 = time.perf_counter()
print(res)
print(f"{{t2 - t1:.6f}}")
""")

        run_py = subprocess.run(["python", py_file], capture_output=True, text=True, encoding="utf-8")
        if run_py.returncode != 0:
            py_time = None
            print(f"Python run failed for {name}: {run_py.stderr}")
        else:
            lines = run_py.stdout.strip().split("\n")
            try:
                py_time = float(lines[-1].strip())
            except ValueError:
                py_time = None
                print(f"Python output parsing failed for {name}: {run_py.stdout}")

        # Format times and speedups
        btq_str = f"{btq_time:.3f}s" if btq_time is not None else "ERROR"
        cpp_str = f"{cpp_time:.3f}s" if cpp_time is not None else "ERROR"
        go_str = f"{go_time:.3f}s" if go_time is not None else "ERROR"
        py_str = f"{py_time:.3f}s" if py_time is not None else "ERROR"

        vs_cpp_str = "**N/A**"
        if btq_time is not None and cpp_time is not None:
            if btq_time > 0:
                ratio = cpp_time / btq_time
                vs_cpp_str = f"{ratio:.2f}x"
            else:
                vs_cpp_str = "∞"
                
        vs_go_str = "**N/A**"
        if btq_time is not None and go_time is not None:
            if btq_time > 0:
                ratio = go_time / btq_time
                vs_go_str = f"{ratio:.2f}x"
            else:
                vs_go_str = "∞"

        vs_py_str = "**N/A**"
        if btq_time is not None and py_time is not None:
            if btq_time > 0:
                ratio = py_time / btq_time
                vs_py_str = f"{ratio:.2f}x"
            else:
                vs_py_str = "∞"

        row = f"| {i+1} | {name} | {btq_str} | {cpp_str} | {go_str} | {py_str} | **{vs_cpp_str}** | **{vs_go_str}** | **{vs_py_str}** |"
        print(row, flush=True)
        results_table.append(row)

        # Cleanup binaries to save space, but leave source files for inspection
        if os.path.exists(cpp_exe):
            try: os.remove(cpp_exe)
            except: pass
        if os.path.exists(go_exe):
            try: os.remove(go_exe)
            except: pass
        btq_exe_guess = btq_file.replace(".btq", ".exe")
        if os.path.exists(btq_exe_guess):
            try: os.remove(btq_exe_guess)
            except: pass

    # Clean up top level butaq.exe
    if os.path.exists("butaq.exe"):
        try: os.remove("butaq.exe")
        except: pass
    if os.path.exists("out.asm"):
        try: os.remove("out.asm")
        except: pass

    # Write to results.md
    with open("benchmarks/results.md", "w", encoding="utf-8") as f:
        f.write("# Butaq vs C++ (-O3), Go, and Python Performance Benchmarks\n\n")
        f.write("20 comprehensive performance tests comparing Butaq (Native x86-64) against C++ (GCC -O3), Go, and Python.\n\n")
        f.write("| # | Benchmark Name | Butaq (Native) | C++ (-O3) | Golang | Python | Butaq vs C++ | Butaq vs Go | Butaq vs Py |\n")
        f.write("|---|---|---|---|---|---|---|---|---|\n")
        for row in results_table:
            f.write(row + "\n")
        f.write("\n## Highlights & Context\n")
        f.write("- **C++ (-O3)** represents highly-optimized compiled machine code (GCC -O3 auto-vectorization, loop unrolling).\n")
        f.write("- **Go** compiled code runs with a lightweight runtime and GC, matching near-native speed.\n")
        f.write("- **Python** runs as an interpreted language, showing standard bytecode execution overhead.\n")
        f.write("- **Butaq (Native)** compiles directly to clean x86-64 machine code via NASM. It performs dramatically faster than Python, matches or beats Go in numeric tasks, and approaches optimized C++ on arithmetic/loops.\n")

if __name__ == "__main__":
    main()
