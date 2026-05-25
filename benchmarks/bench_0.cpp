#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t fib(int64_t n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = fib(35);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
