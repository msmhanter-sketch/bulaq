#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t collatz(int64_t limit) {
    int64_t total = 0;
    int64_t i = 1;
    while (i < limit) {
        int64_t n = i;
        while (n != 1) {
            int64_t div = n / 2;
            int64_t rem = n - (div * 2);
            if (rem == 0) { n = div; }
            else { n = n * 3 + 1; }
            total += 1;
        }
        i += 1;
    }
    return total;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = collatz(50000);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
