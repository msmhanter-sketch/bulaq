#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t ackermann(int64_t m, int64_t n) {
    if (m == 0) return n + 1;
    if (m > 0 && n == 0) return ackermann(m - 1, 1);
    return ackermann(m - 1, ackermann(m, n - 1));
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = ackermann(3, 7);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
