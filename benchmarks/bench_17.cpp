#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

double golden_ratio(int64_t limit) {
    double a = 1.0;
    double b = 1.0;
    int64_t i = 0;
    while (i < limit) {
        double t = b;
        b = a + b;
        a = t;
        i += 1;
    }
    return b / a;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = golden_ratio(50);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
