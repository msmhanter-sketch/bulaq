#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t string_concat() {
    std::string s = "";
    int64_t i = 0;
    while (i < 2000) {
        s += "a";
        i += 1;
    }
    return s.length();
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = string_concat();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
