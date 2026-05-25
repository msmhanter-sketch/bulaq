#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t type_cast() {
    int64_t c = 0;
    int64_t i = 0;
    while (i < 2000000) {
        c += (int64_t)(1.5);
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = type_cast();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
