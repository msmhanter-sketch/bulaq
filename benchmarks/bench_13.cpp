#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t sum_of_squares(int64_t limit) {
    int64_t c = 0;
    int64_t i = 0;
    while (i < limit) {
        c += (i * i);
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = sum_of_squares(500000);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
