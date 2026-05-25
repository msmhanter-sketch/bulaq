import time
import sys
sys.setrecursionlimit(200000)

def string_concat():
    s = ""
    i = 0
    while i < 2000:
        s += "a"
        i += 1
    return len(s)


t1 = time.perf_counter()
res = string_concat()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
