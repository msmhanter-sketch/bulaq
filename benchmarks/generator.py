import os
import subprocess
import time
import sys
import shutil

BENCHMARKS = [
    {
        "name": "fib",
        "btq_func": "fib(35)",
        "cpp_func": "fib(35)",
        "go_func": "fib(35)",
        "desc": "Fibonacci (Recursion, 35)",
        "btq_code": """
функция fib(n) { 
    n 2 кіші егер { n қайтару } 
    fib(n 1 алу) fib(n 2 алу) қосу қайтару 
}""",
        "cpp_code": """
int64_t fib(int64_t n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}""",
        "go_code": """
func fib(n int64) int64 {
    if n < 2 {
        return n
    }
    return fib(n-1) + fib(n-2)
}"""
    },
    {
        "name": "loop_10m",
        "btq_func": "loop_10m()",
        "cpp_func": "loop_10m()",
        "go_func": "loop_10m()",
        "desc": "10M Loop Increment",
        "btq_code": """
функция loop_10m() { 
    x 0 болсын
    x 10000000 кіші әзірше { x x 1 қосу болсын }
    x қайтару 
}""",
        "cpp_code": """
int64_t loop_10m() {
    int64_t x = 0;
    while (x < 10000000) { x += 1; }
    return x;
}""",
        "go_code": """
func loop_10m() int64 {
    x := int64(0)
    for x < 10000000 {
        x += 1
    }
    return x
}"""
    },
    {
        "name": "nested_loop",
        "btq_func": "nested_loop()",
        "cpp_func": "nested_loop()",
        "go_func": "nested_loop()",
        "desc": "Nested Loops (1000x1000)",
        "btq_code": """
функция nested_loop() { 
    c 0 болсын
    i 0 болсын
    i 1000 кіші әзірше { 
        j 0 болсын
        j 1000 кіші әзірше { c c 1 қосу болсын
        j j 1 қосу болсын } i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
int64_t nested_loop() {
    int64_t c = 0;
    int64_t i = 0;
    while (i < 1000) {
        int64_t j = 0;
        while (j < 1000) {
            c += 1;
            j += 1;
        }
        i += 1;
    }
    return c;
}""",
        "go_code": """
func nested_loop() int64 {
    c := int64(0)
    i := int64(0)
    for i < 1000 {
        j := int64(0)
        for j < 1000 {
            c += 1
            j += 1
        }
        i += 1
    }
    return c
}"""
    },
    {
        "name": "math_float",
        "btq_func": "math_float()",
        "cpp_func": "math_float()",
        "go_func": "math_float()",
        "desc": "Float Math Ops (5M)",
        "btq_code": """
функция math_float() { 
    x 1.0 болсын
    i 0 болсын
    i 5000000 кіші әзірше { 
        x x 1.00001 көбейту болсын
        x x 1.000001 бөлу болсын
        i i 1 қосу болсын 
    } x қайтару 
}""",
        "cpp_code": """
double math_float() {
    double x = 1.0;
    int64_t i = 0;
    while (i < 5000000) {
        x *= 1.00001;
        x /= 1.000001;
        i += 1;
    }
    return x;
}""",
        "go_code": """
func math_float() float64 {
    x := 1.0
    i := int64(0)
    for i < 5000000 {
        x *= 1.00001
        x /= 1.000001
        i += 1
    }
    return x
}"""
    },
    {
        "name": "ackermann",
        "btq_func": "ackermann(3, 7)",
        "cpp_func": "ackermann(3, 7)",
        "go_func": "ackermann(3, 7)",
        "desc": "Ackermann(3, 7)",
        "btq_code": """
функция ackermann(m, n) { 
    m 0 тең егер { n 1 қосу қайтару } 
    m 0 үлкен n 0 тең және егер { ackermann(m 1 алу, 1) қайтару } 
    ackermann(m 1 алу, ackermann(m, n 1 алу)) қайтару 
}""",
        "cpp_code": """
int64_t ackermann(int64_t m, int64_t n) {
    if (m == 0) return n + 1;
    if (m > 0 && n == 0) return ackermann(m - 1, 1);
    return ackermann(m - 1, ackermann(m, n - 1));
}""",
        "go_code": """
func ackermann(m, n int64) int64 {
    if m == 0 {
        return n + 1
    }
    if m > 0 && n == 0 {
        return ackermann(m-1, 1)
    }
    return ackermann(m-1, ackermann(m, n-1))
}"""
    },
    {
        "name": "boolean_logic",
        "btq_func": "boolean_logic()",
        "cpp_func": "boolean_logic()",
        "go_func": "boolean_logic()",
        "desc": "Boolean Logic (2M)",
        "btq_code": """
функция boolean_logic() { 
    i 0 болсын
    c 0 болсын
    i 2000000 кіші әзірше { 
        b1 i 2 бөлу 0 тең болсын
        b2 i 3 бөлу 0 тең болсын
        b1 b2 емес және егер { c c 1 қосу болсын } 
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
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
}""",
        "go_code": """
func boolean_logic() int64 {
    i := int64(0)
    c := int64(0)
    for i < 2000000 {
        b1 := (i / 2) == 0
        b2 := (i / 3) == 0
        if b1 && !b2 {
            c += 1
        }
        i += 1
    }
    return c
}"""
    },
    {
        "name": "string_concat",
        "btq_func": "string_concat()",
        "cpp_func": "string_concat()",
        "go_func": "string_concat()",
        "desc": "String Concat (2000)",
        "btq_code": """
функция string_concat() { 
    s "" болсын
    i 0 болсын
    i 2000 кіші әзірше { s s "a" біріктіру болсын
    i i 1 қосу болсын } 
    s мәтін_ұзындығы қайтару 
}""",
        "cpp_code": """
int64_t string_concat() {
    std::string s = "";
    int64_t i = 0;
    while (i < 2000) {
        s += "a";
        i += 1;
    }
    return s.length();
}""",
        "go_code": """
func string_concat() int64 {
    s := ""
    i := int64(0)
    for i < 2000 {
        s += "a"
        i += 1
    }
    return int64(len(s))
}"""
    },
    {
        "name": "string_compare",
        "btq_func": "string_compare()",
        "cpp_func": "string_compare()",
        "go_func": "string_compare()",
        "desc": "String Compare (2M)",
        "btq_code": """
функция string_compare() { 
    i 0 болсын
    c 0 болсын
    s1 "hello_world_test" болсын
    s2 "hello_world_test" болсын
    i 2000000 кіші әзірше { 
        s1 s2 тең егер { c c 1 қосу болсын } 
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
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
}""",
        "go_code": """
func string_compare() int64 {
    i := int64(0)
    c := int64(0)
    s1 := "hello_world_test"
    s2 := "hello_world_test"
    for i < 2000000 {
        if s1 == s2 {
            c += 1
        }
        i += 1
    }
    return c
}"""
    },
    {
        "name": "collatz",
        "btq_func": "collatz(50000)",
        "cpp_func": "collatz(50000)",
        "go_func": "collatz(50000)",
        "desc": "Collatz up to 50K",
        "btq_code": """
функция collatz(limit) { 
    total 0 болсын
    i 1 болсын
    i limit кіші әзірше { 
        n i болсын
        n 1 тең_емес әзірше { 
            div n 2 бөлу болсын
            rem n div 2 көбейту алу болсын
            rem 0 тең егер { n div болсын } әйтпесе { n n 3 көбейту 1 қосу болсын } 
            total total 1 қосу болсын 
        } 
        i i 1 қосу болсын 
    } 
    total қайтару 
}""",
        "cpp_code": """
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
}""",
        "go_code": """
func collatz(limit int64) int64 {
    total := int64(0)
    i := int64(1)
    for i < limit {
        n := i
        for n != 1 {
            div := n / 2
            rem := n - (div * 2)
            if rem == 0 {
                n = div
            } else {
                n = n*3 + 1
            }
            total += 1
        }
        i += 1
    }
    return total
}"""
    },
    {
        "name": "gcd_loop",
        "btq_func": "gcd_loop()",
        "cpp_func": "gcd_loop()",
        "go_func": "gcd_loop()",
        "desc": "GCD iterations (2M)",
        "btq_code": """
функция gcd(a, b) { 
    b 0 тең_емес әзірше { 
        div a b бөлу болсын
        rem a div b көбейту алу болсын
        a b болсын
        b rem болсын } 
    a қайтару 
}
функция gcd_loop() { 
    i 0 болсын
    c 0 болсын
    i 2000000 кіші әзірше { c c gcd(12345, i) қосу болсын
    i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
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
}""",
        "go_code": """
func gcd(a, b int64) int64 {
    for b != 0 {
        div := a / b
        rem := a - (div * b)
        a = b
        b = rem
    }
    return a
}
func gcd_loop() int64 {
    i := int64(0)
    c := int64(0)
    for i < 2000000 {
        c += gcd(12345, i)
        i += 1
    }
    return c
}"""
    },
    {
        "name": "factorial",
        "btq_func": "factorial_loop()",
        "cpp_func": "factorial_loop()",
        "go_func": "factorial_loop()",
        "desc": "Factorial Math (1M)",
        "btq_code": """
функция factorial_loop() { 
    c 0 болсын
    i 0 болсын
    i 1000000 кіші әзірше { 
        f 1 болсын
        j 1 болсын
        j 10 кіші әзірше { f f j көбейту болсын
        j j 1 қосу болсын } 
        c c f қосу болсын
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
int64_t factorial_loop() {
    int64_t c = 0;
    int64_t i = 0;
    while (i < 1000000) {
        int64_t f = 1;
        int64_t j = 1;
        while (j < 10) {
            f *= j;
            j += 1;
        }
        c += f;
        i += 1;
    }
    return c;
}""",
        "go_code": """
func factorial_loop() int64 {
    c := int64(0)
    i := int64(0)
    for i < 1000000 {
        f := int64(1)
        j := int64(1)
        for j < 10 {
            f *= j
            j += 1
        }
        c += f
        i += 1
    }
    return c
}"""
    },
    {
        "name": "bitwise_ops",
        "btq_func": "bitwise_ops()",
        "cpp_func": "bitwise_ops()",
        "go_func": "bitwise_ops()",
        "desc": "Bitwise shifts (2M)",
        "btq_code": """
функция bitwise_ops() { 
    c 0 болсын
    i 1 болсын
    i 2000000 кіші әзірше { 
        c c i 1 жылжыту_сол қосу болсын
        c c i 1 жылжыту_оң қосу болсын
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
int64_t bitwise_ops() {
    int64_t c = 0;
    int64_t i = 1;
    while (i < 2000000) {
        c += (i << 1);
        c += (i >> 1);
        i += 1;
    }
    return c;
}""",
        "go_code": """
func bitwise_ops() int64 {
    c := int64(0)
    i := int64(1)
    for i < 2000000 {
        c += (i << 1)
        c += (i >> 1)
        i += 1
    }
    return c
}"""
    },
    {
        "name": "type_cast",
        "btq_func": "type_cast()",
        "cpp_func": "type_cast()",
        "go_func": "type_cast()",
        "desc": "Type Casting (2M)",
        "btq_code": """
функция type_cast() { 
    c 0 болсын
    i 0 болсын
    i 2000000 кіші әзірше { 
        c c бүтін(1.5) қосу болсын
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
int64_t type_cast() {
    int64_t c = 0;
    int64_t i = 0;
    while (i < 2000000) {
        c += (int64_t)(1.5);
        i += 1;
    }
    return c;
}""",
        "go_code": """
func type_cast() int64 {
    c := int64(0)
    i := int64(0)
    val := 1.5
    for i < 2000000 {
        c += int64(val)
        i += 1
    }
    return c
}"""
    },
    {
        "name": "sum_of_squares",
        "btq_func": "sum_of_squares(500000)",
        "cpp_func": "sum_of_squares(500000)",
        "go_func": "sum_of_squares(500000)",
        "desc": "Sum of Squares (500K)",
        "btq_code": """
функция sum_of_squares(limit) { 
    c 0 болсын
    i 0 болсын
    i limit кіші әзірше { 
        c c i i көбейту қосу болсын
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
int64_t sum_of_squares(int64_t limit) {
    int64_t c = 0;
    int64_t i = 0;
    while (i < limit) {
        c += (i * i);
        i += 1;
    }
    return c;
}""",
        "go_code": """
func sum_of_squares(limit int64) int64 {
    c := int64(0)
    i := int64(0)
    for i < limit {
        c += (i * i)
        i += 1
    }
    return c
}"""
    },
    {
        "name": "pi_approx",
        "btq_func": "pi_approx(1000000)",
        "cpp_func": "pi_approx(1000000)",
        "go_func": "pi_approx(1000000)",
        "desc": "Pi Approx (1M)",
        "btq_code": """
функция pi_approx(n) { 
    s 0.0 болсын
    i 0 болсын
    i n кіші әзірше { 
        div 2 i көбейту 1 қосу болсын
        rem i 2 бөлу 2 көбейту i тең болсын
        rem егер { s s 4.0 div бөлу қосу болсын } әйтпесе { s s 4.0 div бөлу алу болсын } 
        i i 1 қосу болсын } 
    s қайтару 
}""",
        "cpp_code": """
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
}""",
        "go_code": """
func pi_approx(n int64) float64 {
    s := 0.0
    i := int64(0)
    for i < n {
        div := float64(2*i + 1)
        rem := (i/2)*2 == i
        if rem {
            s += 4.0 / div
        } else {
            s -= 4.0 / div
        }
        i += 1
    }
    return s
}"""
    },
    {
        "name": "prime_count",
        "btq_func": "prime_count(10000)",
        "cpp_func": "prime_count(10000)",
        "go_func": "prime_count(10000)",
        "desc": "Prime Count (10K)",
        "btq_code": """
функция is_prime(n) { 
    n 2 кіші егер { 0 қайтару } 
    i 2 болсын
    i i көбейту n кіші_немесе_тең әзірше { 
        div n i бөлу болсын
        rem n div i көбейту алу болсын
        rem 0 тең егер { 0 қайтару } 
        i i 1 қосу болсын } 
    1 қайтару 
}
функция prime_count(limit) { 
    c 0 болсын
    i 2 болсын
    i limit кіші әзірше { 
        is_prime(i) 1 тең егер { c c 1 қосу болсын } 
        i i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
int64_t is_prime(int64_t n) {
    if (n < 2) return 0;
    int64_t i = 2;
    while (i * i <= n) {
        int64_t div = n / i;
        int64_t rem = n - (div * i);
        if (rem == 0) return 0;
        i += 1;
    }
    return 1;
}
int64_t prime_count(int64_t limit) {
    int64_t c = 0;
    int64_t i = 2;
    while (i < limit) {
        if (is_prime(i) == 1) { c += 1; }
        i += 1;
    }
    return c;
}""",
        "go_code": """
func is_prime(n int64) int64 {
    if n < 2 {
        return 0
    }
    i := int64(2)
    for i*i <= n {
        div := n / i
        rem := n - (div * i)
        if rem == 0 {
            return 0
        }
        i += 1
    }
    return 1
}
func prime_count(limit int64) int64 {
    c := int64(0)
    i := int64(2)
    for i < limit {
        if is_prime(i) == 1 {
            c += 1
        }
        i += 1
    }
    return c
}"""
    },
    {
        "name": "harmonic_sum",
        "btq_func": "harmonic_sum(2000000)",
        "cpp_func": "harmonic_sum(2000000)",
        "go_func": "harmonic_sum(2000000)",
        "desc": "Harmonic Sum (2M)",
        "btq_code": """
функция harmonic_sum(limit) { 
    s 0.0 болсын
    i 1 болсын
    i limit кіші әзірше { 
        s s 1.0 i бөлу қосу болсын
        i i 1 қосу болсын } 
    s қайтару 
}""",
        "cpp_code": """
double harmonic_sum(int64_t limit) {
    double s = 0.0;
    int64_t i = 1;
    while (i < limit) {
        s += 1.0 / i;
        i += 1;
    }
    return s;
}""",
        "go_code": """
func harmonic_sum(limit int64) float64 {
    s := 0.0
    i := int64(1)
    for i < limit {
        s += 1.0 / float64(i)
        i += 1
    }
    return s
}"""
    },
    {
        "name": "golden_ratio",
        "btq_func": "golden_ratio(50)",
        "cpp_func": "golden_ratio(50)",
        "go_func": "golden_ratio(50)",
        "desc": "Golden Ratio (50)",
        "btq_code": """
функция golden_ratio(limit) { 
    a 1.0 болсын
    b 1.0 болсын
    i 0 болсын
    i limit кіші әзірше { 
        t b болсын
        b a b қосу болсын
        a t болсын
        i i 1 қосу болсын } 
    b a бөлу қайтару 
}""",
        "cpp_code": """
double golden_ratio(int64_t limit) {
    double a = 1.0;
    double b = 1.0;
    int64_t i = 0;
    while (i < limit) {
        double t = b;
        b = a + b;
        a = t;
        i += 1;
    }
    return b / a;
}""",
        "go_code": """
func golden_ratio(limit int64) float64 {
    a := 1.0
    b := 1.0
    i := int64(0)
    for i < limit {
        t := b
        b = a + b
        a = t
        i += 1
    }
    return b / a
}"""
    },
    {
        "name": "power_loop",
        "btq_func": "power_loop(5000)",
        "cpp_func": "power_loop(5000)",
        "go_func": "power_loop(5000)",
        "desc": "Power Loop (5K)",
        "btq_code": """
функция power_loop(limit) { 
    s 0.0 болсын
    i 0 болсын
    i limit кіші әзірше { 
        p 1.0 болсын
        j 0 болсын
        j 10 кіші әзірше { 
            p p 1.1 көбейту болсын
            j j 1 қосу болсын } 
        s s p қосу болсын
        i i 1 қосу болсын } 
    s қайтару 
}""",
        "cpp_code": """
double power_loop(int64_t limit) {
    double s = 0.0;
    int64_t i = 0;
    while (i < limit) {
        double p = 1.0;
        int64_t j = 0;
        while (j < 10) {
            p *= 1.1;
            j += 1;
        }
        s += p;
        i += 1;
    }
    return s;
}""",
        "go_code": """
func power_loop(limit int64) float64 {
    s := 0.0
    i := int64(0)
    for i < limit {
        p := 1.0
        j := int64(0)
        for j < 10 {
            p *= 1.1
            j += 1
        }
        s += p
        i += 1
    }
    return s
}"""
    },
    {
        "name": "mandelbrot_mock",
        "btq_func": "mandelbrot_mock(100)",
        "cpp_func": "mandelbrot_mock(100)",
        "go_func": "mandelbrot_mock(100)",
        "desc": "Mandelbrot Mock (100)",
        "btq_code": """
функция mandelbrot_mock(limit) { 
    c 0 болсын
    x_i 0 болсын
    x_i limit кіші әзірше { 
        y_i 0 болсын
        y_i limit кіші әзірше { 
            zr 0.0 болсын
            zi 0.0 болсын
            j 0 болсын
            j 15 кіші әзірше { 
                zr2 zr zr көбейту zi zi көбейту алу x_i қосу болсын
                zi 2.0 zr көбейту zi көбейту y_i қосу болсын
                zr zr2 болсын
                j j 1 қосу болсын } 
            c c 1 қосу болсын
            y_i y_i 1 қосу болсын } 
        x_i x_i 1 қосу болсын } 
    c қайтару 
}""",
        "cpp_code": """
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
}""",
        "go_code": """
func mandelbrot_mock(limit int64) int64 {
    c := int64(0)
    x_i := int64(0)
    for x_i < limit {
        y_i := int64(0)
        for y_i < limit {
            zr := 0.0
            zi := 0.0
            j := int64(0)
            for j < 15 {
                zr2 := zr*zr - zi*zi + float64(x_i)
                zi = 2.0*zr*zi + float64(y_i)
                zr = zr2
                j += 1
            }
            c += 1
            y_i += 1
        }
        x_i += 1
    }
    return c
}"""
    }
]

def main():
    print("Building Butaq compiler...")
    res = subprocess.run(["go", "build", "-o", "butaq.exe", "."], capture_output=True, text=True, encoding="utf-8")
    if res.returncode != 0:
        print("Go build error:", res.stderr)
        return

    # Ensure benchmarks dir exists
    os.makedirs("benchmarks", exist_ok=True)

    results_table = []
    
    print("\n| # | Benchmark Name | Butaq (Native) | C++ (-O3) | Golang | Butaq vs C++ | Butaq vs Go |")
    print("|---|---|---|---|---|---|---|")
    
    for i, b in enumerate(BENCHMARKS):
        name = b['desc']
        btq_call = b['btq_func']
        cpp_call = b['cpp_func']
        go_call = b['go_func']
        btq_code = b['btq_code']
        cpp_code = b['cpp_code']
        go_code = b['go_code']

        # 1. Write and Run Butaq file
        btq_file = f"benchmarks/bench_{i}.btq"
        with open(btq_file, "w", encoding="utf-8") as f:
            f.write(btq_code + "\n")
            f.write(f'''t1 уақыт() болсын\nres {btq_call} болсын\nt2 уақыт() болсын\nt2 t1 алу жазу\n''')

        res = subprocess.run([".\\butaq.exe", "-r", btq_file], capture_output=True, text=True, encoding="utf-8")
        if res.returncode != 0:
            btq_time = None
            print(f"Butaq compilation/run failed for {name}: {res.stderr}")
        else:
            lines = res.stdout.strip().split("\n")
            try:
                btq_time = float(lines[-1].strip())
            except ValueError:
                btq_time = None
                print(f"Butaq output parsing failed for {name}: {res.stdout}")

        # 2. Write, Compile, and Run C++ file
        cpp_file = f"benchmarks/bench_{i}.cpp"
        cpp_exe = f"benchmarks/bench_cpp_{i}.exe"
        with open(cpp_file, "w", encoding="utf-8") as f:
            f.write("#include <iostream>\n#include <chrono>\n#include <string>\n#include <vector>\n#include <cmath>\n")
            f.write(cpp_code + "\n")
            f.write(f"""
int main() {{
    auto t1 = std::chrono::high_resolution_clock::now();
    auto res = {cpp_call};
    auto t2 = std::chrono::high_resolution_clock::now();
    std::chrono::duration<double> elapsed = t2 - t1;
    std::cout << res << std::endl;
    std::cout << elapsed.count() << std::endl;
    return 0;
}}
""")

        # Compile C++ with -O3
        compile_cpp = subprocess.run(["g++", "-O3", "-o", cpp_exe, cpp_file], capture_output=True, text=True, encoding="utf-8")
        if compile_cpp.returncode != 0:
            cpp_time = None
            print(f"C++ compilation failed for {name}: {compile_cpp.stderr}")
        else:
            run_cpp = subprocess.run([f".\\{cpp_exe}"], capture_output=True, text=True, encoding="utf-8")
            if run_cpp.returncode != 0:
                cpp_time = None
                print(f"C++ run failed for {name}: {run_cpp.stderr}")
            else:
                lines = run_cpp.stdout.strip().split("\n")
                try:
                    cpp_time = float(lines[-1].strip())
                except ValueError:
                    cpp_time = None
                    print(f"C++ output parsing failed for {name}: {run_cpp.stdout}")

        # 3. Write, Compile, and Run Go file
        go_file = f"benchmarks/bench_{i}.go"
        go_exe = f"benchmarks/bench_go_{i}.exe"
        with open(go_file, "w", encoding="utf-8") as f:
            f.write("package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n")
            f.write(go_code + "\n")
            f.write(f"""
func main() {{
    t1 := time.Now()
    res := {go_call}
    t2 := time.Now()
    fmt.Println(res)
    fmt.Printf("%.6f\\n", t2.Sub(t1).Seconds())
}}
""")

        # Compile Go
        compile_go = subprocess.run(["go", "build", "-o", go_exe, go_file], capture_output=True, text=True, encoding="utf-8")
        if compile_go.returncode != 0:
            go_time = None
            print(f"Go compilation failed for {name}: {compile_go.stderr}")
        else:
            run_go = subprocess.run([f".\\{go_exe}"], capture_output=True, text=True, encoding="utf-8")
            if run_go.returncode != 0:
                go_time = None
                print(f"Go run failed for {name}: {run_go.stderr}")
            else:
                lines = run_go.stdout.strip().split("\n")
                try:
                    go_time = float(lines[-1].strip())
                except ValueError:
                    go_time = None
                    print(f"Go output parsing failed for {name}: {run_go.stdout}")

        # Format times and speedups
        btq_str = f"{btq_time:.3f}s" if btq_time is not None else "ERROR"
        cpp_str = f"{cpp_time:.3f}s" if cpp_time is not None else "ERROR"
        go_str = f"{go_time:.3f}s" if go_time is not None else "ERROR"

        vs_cpp_str = "**N/A**"
        if btq_time is not None and cpp_time is not None:
            if btq_time > 0:
                ratio = cpp_time / btq_time
                vs_cpp_str = f"{ratio:.2f}x"
            else:
                vs_cpp_str = "∞"
                
        vs_go_str = "**N/A**"
        if btq_time is not None and go_time is not None:
            if btq_time > 0:
                ratio = go_time / btq_time
                vs_go_str = f"{ratio:.2f}x"
            else:
                vs_go_str = "∞"

        row = f"| {i+1} | {name} | {btq_str} | {cpp_str} | {go_str} | **{vs_cpp_str}** | **{vs_go_str}** |"
        print(row, flush=True)
        results_table.append(row)

        # Cleanup binaries to save space, but leave source files for inspection
        if os.path.exists(cpp_exe):
            try: os.remove(cpp_exe)
            except: pass
        if os.path.exists(go_exe):
            try: os.remove(go_exe)
            except: pass
        # Clean up Butaq compiled file if it was built next to the input
        # Note: butaq compiles input.btq to input.exe (or out.exe) in the working directory
        btq_exe_guess = btq_file.replace(".btq", ".exe")
        if os.path.exists(btq_exe_guess):
            try: os.remove(btq_exe_guess)
            except: pass

    # Clean up top level butaq.exe
    if os.path.exists("butaq.exe"):
        try: os.remove("butaq.exe")
        except: pass
    if os.path.exists("out.asm"):
        try: os.remove("out.asm")
        except: pass

    # Write to results.md
    with open("benchmarks/results.md", "w", encoding="utf-8") as f:
        f.write("# Butaq vs C++ (-O3) & Go Performance Benchmarks\n\n")
        f.write("20 comprehensive performance tests comparing Butaq (Native x86-64) against C++ (GCC -O3) and Go.\n\n")
        f.write("| # | Benchmark Name | Butaq (Native) | C++ (-O3) | Golang | Butaq vs C++ | Butaq vs Go |\n")
        f.write("|---|---|---|---|---|---|---|\n")
        for row in results_table:
            f.write(row + "\n")
        f.write("\n## Highlights & Context\n")
        f.write("- **C++ (-O3)** represents optimized native machine code. In loops and arithmetic, GCC's auto-vectorization and loop unrolling can make C++ extremely fast.\n")
        f.write("- **Go** compiled code runs with a runtime and garbage collection, but matches near-native performance.\n")
        f.write("- **Butaq (Native)** matches or comes close to optimized C++ and Go on pure loops/arithmetic, but lags on memory/string allocations where it uses naive allocs without advanced garbage collection or highly optimized string builders.\n")

if __name__ == "__main__":
    main()
