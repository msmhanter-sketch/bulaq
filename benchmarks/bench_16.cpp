#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

double harmonic_sum(int64_t limit) {
    double s = 0.0;
    int64_t i = 1;
    while (i < limit) {
        s += 1.0 / i;
        i += 1;
    }
    return s;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = harmonic_sum(2000000);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
