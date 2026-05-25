# Butaq Standard Library Reference

> **Butaq** is a programming language with Kazakh-language syntax that compiles to native x86-64 machine code.  
> All functions are built into the language core — no external dependencies.

---

## Table of Contents

1. [String Functions](#string-functions)
2. [Math Functions](#math-functions)
3. [Type Conversion](#type-conversion)
4. [Time Functions](#time-functions)
5. [System Functions](#system-functions)
6. [JSON Functions](#json-functions)
7. [Crypto Functions](#crypto-functions)
8. [Thread Functions](#thread-functions)
9. [File Functions](#file-functions)
10. [I/O](#io)

---

## String Functions

### `мәтін_ұзындығы(str)` → NUMBER
Returns the byte length of a string.

```butaq
len мәтін_ұзындығы("Hello") болсын
len жазу   # → 5
```

### `таңба(code)` → STRING
Creates a single character from a Unicode code point.

```butaq
ch таңба(65) болсын   # → "A"
ch жазу
```

---

## Math Functions

### `кездейсоқ()` → NUMBER
Returns a random float between 0.0 and 1.0.

```butaq
r кездейсоқ() болсын
r жазу   # → 0.374821...
```

### `түбір(x)` → NUMBER
Returns the square root of `x`.

```butaq
result түбір(16.0) болсын
result жазу   # → 4.0
```

### `дәреже(base, exp)` → NUMBER
Returns `base ^ exp`.

```butaq
result дәреже(2.0, 8.0) болсын
result жазу   # → 256.0
```

### `синус(x)` → NUMBER
Returns the sine of `x` (in radians).

### `косинус(x)` → NUMBER
Returns the cosine of `x` (in radians).

---

## Type Conversion

### `мәтін(x)` → STRING
Converts a number to string.

```butaq
s мәтін(42.0) болсын
s жазу   # → "42"
```

### `сан(s)` → NUMBER
Converts a string to number. Returns 0 for invalid input.

```butaq
n сан("3.14") болсын
n жазу   # → 3.14
```

### `бүтін(x)` → INT
Truncates a float to integer.

```butaq
i бүтін(3.99) болсын
i жазу   # → 3
```

---

## Time Functions

### `уақыт()` → NUMBER
Returns the current Unix timestamp as a double (seconds since epoch).

```butaq
t уақыт() болсын
t жазу   # → 1748000000.0
```

### `уақыт_мәтіні(format)` → STRING
Returns the current date/time as a string using `strftime` format codes.

| Format | Description | Example |
|--------|-------------|---------|
| `%Y`   | 4-digit year | `2026` |
| `%m`   | Month (01–12) | `05` |
| `%d`   | Day (01–31) | `20` |
| `%H`   | Hour (24h) | `21` |
| `%M`   | Minute | `09` |
| `%S`   | Second | `45` |

```butaq
now уақыт_мәтіні("%Y-%m-%d %H:%M:%S") болсын
now жазу   # → "2026-05-20 21:09:45"
```

### `ұйықтау(seconds)` → void
Pauses execution for the given number of seconds. Supports fractional values.

```butaq
"Loading..." жазу
ұйықтау(0.5)   # wait 500ms
"Done!" жазу
```

**Benchmark example:**
```butaq
t1 уақыт() болсын
# ... some work ...
ұйықтау(1.0)
t2 уақыт() болсын
elapsed t2 t1 алу болсын
elapsed жазу   # → ~1.0
```

---

## System Functions

### `жүйе(command)` → NUMBER
Runs a shell command. Returns exit code.

```butaq
code жүйе("mkdir output") болсын
```

### `жүйе_шығысы(command)` → STRING
Runs a shell command and captures its stdout as a string.

```butaq
result жүйе_шығысы("echo Hello") болсын
result жазу   # → "Hello"
```

### `аргумент_саны()` → NUMBER
Returns the number of command-line arguments.

### `аргумент(index)` → STRING
Returns the command-line argument at the given index (0-based).

---

## JSON Functions

### `жсон_оқу(text)` → JSON
Parses a JSON string into an object.

### `жсон_жазу(obj)` → STRING
Serializes a JSON object to string.

### `жсон_сан_алу(obj, key)` → NUMBER
Reads a numeric field.

### `жсон_мәтін_алу(obj, key)` → STRING
Reads a string field.

### `жсон_логика_алу(obj, key)` → BOOL
Reads a boolean field.

### `жсон_тізім_өлшемі(list)` → NUMBER
Returns the length of a JSON array.

### `жсон_тізім_элементі(list, index)` → JSON
Gets element at index from a JSON array.

### `жсон_жаңа()` → JSON
Creates a new empty JSON object.

### `жсон_сан_қосу(obj, key, value)` / `жсон_мәтін_қосу` / `жсон_логика_қосу`
Add fields to a JSON object.

**Full example:**
```butaq
person жсон_жаңа() болсын
жсон_мәтін_қосу(person, "name", "Arman")
жсон_сан_қосу(person, "age", 30.0)
жсон_логика_қосу(person, "active", ақиқат)

json_str жсон_жазу(person) болсын
json_str жазу   # → {"name":"Arman","age":30,"active":true}
```

---

## Crypto Functions

### `мд5(text)` → STRING
Returns the MD5 hash (32 hex characters).

### `ша256(text)` → STRING
Returns the SHA-256 hash (64 hex characters).

### `б64_кодтау(text)` → STRING
Base64 encodes the string.

### `б64_декодтау(text)` → STRING
Base64 decodes the string.

```butaq
encoded б64_кодтау("Hello Butaq") болсын
decoded б64_декодтау(encoded) болсын
decoded жазу   # → "Hello Butaq"
```

---

## Thread Functions

### `ұшыру func` — keyword
Spawns a new thread running `func`. Returns a thread handle.

### `ағын_күту(handle)` → NUMBER
Waits for a thread to finish.

```butaq
функция жұмыс() {
    "Thread running" жазу
}

handle ұшыру жұмыс болсын
ағын_күту(handle)
```

---

## File Functions

### `файл_бар_ма(path)` → NUMBER
Returns 1 if file exists, 0 otherwise.

```butaq
exists файл_бар_ма("config.json") болсын
егер exists тең 1.0 {
    "Config found" жазу
}
```

### `файл_жою(path)` → NUMBER
Deletes a file. Returns 0 on success.

---

## I/O

### `жазу` — statement keyword
Prints any value to stdout followed by a newline. Works with all types.

```butaq
"Hello!" жазу
42 жазу
3.14 жазу
ақиқат жазу
```

### `енгізу()` → STRING
Reads a line from stdin (up to newline).

```butaq
"Enter your name: " жазу
name енгізу() болсын
"Hello, " жазу
name жазу
```

---

## Operator Reference

| Operator | Example | Description |
|----------|---------|-------------|
| `қосу` | `x y қосу` | Addition |
| `алу` | `x y алу` | Subtraction |
| `көбейту` | `x y көбейту` | Multiplication |
| `бөлу` | `x y бөлу` | Division |
| `қалдық` | `x y қалдық` | Modulo |
| `тең` | `x тең y` | Equal |
| `тең_емес` | `x тең_емес y` | Not equal |
| `кіші` | `x кіші y` | Less than |
| `үлкен` | `x үлкен y` | Greater than |
| `кіші_тең` | `x кіші_тең y` | Less than or equal |
| `үлкен_тең` | `x үлкен_тең y` | Greater than or equal |
| `және` | `a және b` | Logical AND |
| `немесе` | `a немесе b` | Logical OR |
| `емес` | `емес a` | Logical NOT |

---

## Keyword Reference

| Keyword | Meaning |
|---------|---------|
| `болсын` | Variable assignment (`let`) |
| `жазу` | Print to stdout |
| `егер` | If |
| `әйтпесе` | Else |
| `әзірше` | While loop |
| `функция` | Function definition |
| `қайтару` | Return |
| `құрылым` | Struct definition |
| `жасау` | Struct instantiation |
| `ақиқат` | `true` |
| `жалған` | `false` |
| `ұшыру` | Spawn thread |
| `тоқтату` | Break |
| `жалғастыру` | Continue |
| `импорт` | Import file |
| `әлсіз` | Weak reference (for avoiding cycles) |
