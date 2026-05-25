# Butaq Language Tutorial

Butaq is a modern programming language utilizing Kazakh-inspired syntax and a strict SOV (Subject-Object-Verb) structure. This means the action (verb) always goes at the end of the sentence or statement.

---

## 1. Hello, World!

Your first program in Butaq:
```butaq
"Hello, World!" жазу
```

Where:
- `"Hello, World!"` — the text to print.
- `жазу` — the print instruction (verb at the end).

---

## 2. Variables and Types

To declare or assign variables, use the `болсын` keyword:
```butaq
жас 25 болсын
аты "Alikhan" болсын
бағасы 99.9 болсын
оқушы_ма ақиқат болсын
```

Types are inferred automatically:
- `жас` (age) — `БҮТІН` (int64)
- `аты` (name) — `МӘТІН` (string)
- `бағасы` (price) — `САН` (double)
- `оқушы_ма` (is student) — `АҚИҚАТ` (bool)

---

## 3. Conditionals (егер / әйтпесе)

```butaq
жас 18 болсын

егер жас үлкен_тең 18 {
    "You are an adult!" жазу
} әйтпесе {
    "You are a minor!" жазу
}
```

---

## 4. Loops (әзірше)

```butaq
санауыш 1 болсын

әзірше санауыш кіші_тең 5 {
    санауыш жазу
    санауыш санауыш қосу 1 болсын
}
```

---

## 5. Functions

Functions are defined using `функция` and return values with `қайтару`:
```butaq
функция қосу_екі(сан1, san2) {
    сан1 san2 қосу қайтару
}

нәтиже қосу_екі(10, 20) болсын
нәтиже жазу
```

---

## 6. Structs and Weak References (`әлсіз`)

Use weak references (`әлсіз`) to avoid reference cycles and memory leaks:
```butaq
құрылым Адам {
    аты МӘТІН
    әлсіз дос Адам
}

әли жасау Адам болсын
әли.аты "Ali" болсын

асан жасау Адам болсын
асан.аты "Asan" болсын

# Cycle (resolved by weak reference, memory will be freed automatically)
әли.дос асан болсын
асан.дос әли болсын
```
