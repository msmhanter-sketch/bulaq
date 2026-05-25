#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t nested_loop() {
    int64_t c = 0;
    int64_t i = 0;
    while (i < 1000) {
        int64_t j = 0;
        while (j < 1000) {
            c += 1;
            j += 1;
        }
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = nested_loop();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
