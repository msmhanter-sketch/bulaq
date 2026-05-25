#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

double math_float() {
    double x = 1.0;
    int64_t i = 0;
    while (i < 5000000) {
        x *= 1.00001;
        x /= 1.000001;
        i += 1;
    }
    return x;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = math_float();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
