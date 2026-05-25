#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t boolean_logic() {
    int64_t i = 0;
    int64_t c = 0;
    while (i < 2000000) {
        bool b1 = (i / 2) == 0;
        bool b2 = (i / 3) == 0;
        if (b1 && !b2) { c += 1; }
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = boolean_logic();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
