#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t is_prime(int64_t n) {
    if (n < 2) return 0;
    int64_t i = 2;
    while (i * i <= n) {
        int64_t div = n / i;
        int64_t rem = n - (div * i);
        if (rem == 0) return 0;
        i += 1;
    }
    return 1;
}
int64_t prime_count(int64_t limit) {
    int64_t c = 0;
    int64_t i = 2;
    while (i < limit) {
        if (is_prime(i) == 1) { c += 1; }
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = prime_count(10000);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
