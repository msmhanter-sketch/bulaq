#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t loop_10m() {
    int64_t x = 0;
    while (x < 10000000) { x += 1; }
    return x;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = loop_10m();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
