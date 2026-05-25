#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t factorial_loop() {
    int64_t c = 0;
    int64_t i = 0;
    while (i < 1000000) {
        int64_t f = 1;
        int64_t j = 1;
        while (j < 10) {
            f *= j;
            j += 1;
        }
        c += f;
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = factorial_loop();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
