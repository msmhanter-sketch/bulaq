import time
import sys
sys.setrecursionlimit(200000)

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


t1 = time.perf_counter()
res = string_compare()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
