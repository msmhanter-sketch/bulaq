#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t gcd(int64_t a, int64_t b) {
    while (b != 0) {
        int64_t div = a / b;
        int64_t rem = a - (div * b);
        a = b;
        b = rem;
    }
    return a;
}
int64_t gcd_loop() {
    int64_t i = 0;
    int64_t c = 0;
    while (i < 2000000) {
        c += gcd(12345, i);
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = gcd_loop();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
