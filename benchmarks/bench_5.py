import time
import sys
sys.setrecursionlimit(200000)

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


t1 = time.perf_counter()
res = boolean_logic()
t2 = time.perf_counter()
print(res)
print(f"{t2 - t1:.6f}")
