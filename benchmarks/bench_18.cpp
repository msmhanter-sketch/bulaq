#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

double power_loop(int64_t limit) {
    double s = 0.0;
    int64_t i = 0;
    while (i < limit) {
        double p = 1.0;
        int64_t j = 0;
        while (j < 10) {
            p *= 1.1;
            j += 1;
        }
        s += p;
        i += 1;
    }
    return s;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = power_loop(5000);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
