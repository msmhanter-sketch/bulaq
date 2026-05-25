#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t mandelbrot_mock(int64_t limit) {
    int64_t c = 0;
    int64_t x_i = 0;
    while (x_i < limit) {
        int64_t y_i = 0;
        while (y_i < limit) {
            double zr = 0.0;
            double zi = 0.0;
            int64_t j = 0;
            while (j < 15) {
                double zr2 = zr * zr - zi * zi + x_i;
                zi = 2.0 * zr * zi + y_i;
                zr = zr2;
                j += 1;
            }
            c += 1;
            y_i += 1;
        }
        x_i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = mandelbrot_mock(100);
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
