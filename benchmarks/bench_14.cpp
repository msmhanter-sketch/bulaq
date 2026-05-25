#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

double pi_approx(int64_t n) {
    double s = 0.0;
    int64_t i = 0;
    while (i < n) {
        double div = 2 * i + 1;
        bool rem = (i / 2) * 2 == i;
        if (rem) { s += 4.0 / div; }
        else { s -= 4.0 / div; }
        i += 1;
    }
    return s;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = pi_approx(1000000);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
