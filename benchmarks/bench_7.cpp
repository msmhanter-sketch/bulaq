#include <iostream>
#include <chrono>
#include <string>
#include <vector>
#include <cmath>

int64_t string_compare() {
    int64_t i = 0;
    int64_t c = 0;
    std::string s1 = "hello_world_test";
    std::string s2 = "hello_world_test";
    while (i < 2000000) {
        if (s1 == s2) { c += 1; }
        i += 1;
    }
    return c;
}

int main() {
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = string_compare();
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}
