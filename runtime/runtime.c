#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

#if defined(_WIN32) || defined(_WIN64)
#include <windows.h>
double _runtime_clock() {
    LARGE_INTEGER freq, count;
    QueryPerformanceFrequency(&freq);
    QueryPerformanceCounter(&count);
    return (double)count.QuadPart / (double)freq.QuadPart;
}
#else
#include <pthread.h>
#include <unistd.h>
#include <time.h>
double _runtime_clock() {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return ts.tv_sec + ts.tv_nsec / 1e9;
}
#endif

static inline int64_t atomic_increment(int64_t* val) {
#if defined(_MSC_VER)
    return InterlockedIncrement64((volatile LONG64*)val);
#else
    return __sync_add_and_fetch(val, 1);
#endif
}

static inline int64_t atomic_decrement(int64_t* val) {
#if defined(_MSC_VER)
    return InterlockedDecrement64((volatile LONG64*)val);
#else
    return __sync_sub_and_fetch(val, 1);
#endif
}

// =============================================================================
// 1. Core ARC & Weak References (Zeroing Side Table Map)
// =============================================================================

typedef struct WeakCell {
    void* obj;
    int64_t weak_count;
} WeakCell;

typedef struct {
    int64_t strong_count;
    void (*destructor)(void*);
} RefHeader;

#define HASH_SIZE 1024
typedef struct HashEntry {
    void* key;
    WeakCell* value;
    struct HashEntry* next;
} HashEntry;

static HashEntry* weak_table[HASH_SIZE];

#if defined(_WIN32) || defined(_WIN64)
static CRITICAL_SECTION weak_mutex;
static int weak_mutex_initialized = 0;
#define WEAK_LOCK() { if(!weak_mutex_initialized) { InitializeCriticalSection(&weak_mutex); weak_mutex_initialized = 1; } EnterCriticalSection(&weak_mutex); }
#define WEAK_UNLOCK() LeaveCriticalSection(&weak_mutex)
#else
static pthread_mutex_t weak_mutex = PTHREAD_MUTEX_INITIALIZER;
#define WEAK_LOCK() pthread_mutex_lock(&weak_mutex)
#define WEAK_UNLOCK() pthread_mutex_unlock(&weak_mutex)
#endif

static uint32_t hash_ptr(void* ptr) {
    uintptr_t val = (uintptr_t)ptr;
    uint32_t hash = 2166136261U;
    uint8_t* p = (uint8_t*)&val;
    for (size_t i = 0; i < sizeof(val); i++) {
        hash ^= p[i];
        hash *= 16777619U;
    }
    return hash % HASH_SIZE;
}

static WeakCell* weak_map_get(void* key) {
    uint32_t idx = hash_ptr(key);
    HashEntry* entry = weak_table[idx];
    while (entry) {
        if (entry->key == key) {
            return entry->value;
        }
        entry = entry->next;
    }
    return NULL;
}

static void weak_map_set(void* key, WeakCell* value) {
    uint32_t idx = hash_ptr(key);
    HashEntry* entry = weak_table[idx];
    while (entry) {
        if (entry->key == key) {
            entry->value = value;
            return;
        }
        entry = entry->next;
    }
    HashEntry* new_entry = (HashEntry*)malloc(sizeof(HashEntry));
    new_entry->key = key;
    new_entry->value = value;
    new_entry->next = weak_table[idx];
    weak_table[idx] = new_entry;
}

static void weak_map_remove(void* key) {
    uint32_t idx = hash_ptr(key);
    HashEntry* entry = weak_table[idx];
    HashEntry* prev = NULL;
    while (entry) {
        if (entry->key == key) {
            if (prev) {
                prev->next = entry->next;
            } else {
                weak_table[idx] = entry->next;
            }
            free(entry);
            return;
        }
        prev = entry;
        entry = entry->next;
    }
}

void _weak_notify_destroy(void* ptr) {
    if (!ptr) return;
    WEAK_LOCK();
    WeakCell* cell = weak_map_get(ptr);
    if (cell) {
        cell->obj = NULL;
        if (cell->weak_count == 0) {
            free(cell);
        }
        weak_map_remove(ptr);
    }
    WEAK_UNLOCK();
}

void* _alloc_ref(int64_t size, void (*destructor)(void*)) {
    RefHeader* hdr = (RefHeader*)malloc(sizeof(RefHeader) + size);
    if (!hdr) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    hdr->strong_count = 1;
    hdr->destructor = destructor;
    memset((void*)(hdr + 1), 0, size);
    return (void*)(hdr + 1);
}

void _retain(void* ptr) {
    if (!ptr) return;
    RefHeader* hdr = (RefHeader*)ptr - 1;
    if (hdr->strong_count == -1) return; // Static / immortal objects
    atomic_increment(&hdr->strong_count);
}

void _release(void* ptr) {
    if (!ptr) return;
    RefHeader* hdr = (RefHeader*)ptr - 1;
    if (hdr->strong_count == -1) return;
    if (atomic_decrement(&hdr->strong_count) == 0) {
        _weak_notify_destroy(ptr);
        if (hdr->destructor) {
            hdr->destructor(ptr);
        }
        free(hdr);
    }
}

void _weak_assign(void** dest_weak_ptr_addr, void* source_strong_ptr) {
    if (!dest_weak_ptr_addr) return;
    WEAK_LOCK();
    WeakCell* old_cell = (WeakCell*)*dest_weak_ptr_addr;
    if (old_cell) {
        old_cell->weak_count--;
        if (old_cell->weak_count == 0 && old_cell->obj == NULL) {
            free(old_cell);
        }
        *dest_weak_ptr_addr = NULL;
    }
    if (source_strong_ptr) {
        WeakCell* cell = weak_map_get(source_strong_ptr);
        if (!cell) {
            cell = (WeakCell*)malloc(sizeof(WeakCell));
            cell->obj = source_strong_ptr;
            cell->weak_count = 1;
            weak_map_set(source_strong_ptr, cell);
        } else {
            cell->weak_count++;
        }
        *dest_weak_ptr_addr = (void*)cell;
    }
    WEAK_UNLOCK();
}

void* _weak_load(void** weak_ptr_addr) {
    if (!weak_ptr_addr || !*weak_ptr_addr) return NULL;
    WEAK_LOCK();
    WeakCell* cell = (WeakCell*)*weak_ptr_addr;
    void* obj = cell->obj;
    if (obj) {
        _retain(obj);
    }
    WEAK_UNLOCK();
    return obj;
}

void _weak_clear(void** weak_ptr_addr) {
    if (!weak_ptr_addr || !*weak_ptr_addr) return;
    WEAK_LOCK();
    WeakCell* cell = (WeakCell*)*weak_ptr_addr;
    cell->weak_count--;
    if (cell->weak_count == 0 && cell->obj == NULL) {
        free(cell);
    }
    *weak_ptr_addr = NULL;
    WEAK_UNLOCK();
}

// =============================================================================
// 2. Cryptography Functions (MD5, SHA-256, Base64)
// =============================================================================

// --- MD5 ---
typedef struct {
    uint32_t state[4];
    uint32_t count[2];
    uint8_t buffer[64];
} MD5_CTX;

static void MD5Transform(uint32_t state[4], const uint8_t block[64]);

static void MD5Init(MD5_CTX* context) {
    context->count[0] = context->count[1] = 0;
    context->state[0] = 0x67452301;
    context->state[1] = 0xefcdab89;
    context->state[2] = 0x98badcfe;
    context->state[3] = 0x10325476;
}

static void MD5Update(MD5_CTX* context, const uint8_t* input, uint32_t inputLen) {
    uint32_t i, index, partLen;
    index = (uint32_t)((context->count[0] >> 3) & 0x3F);
    if ((context->count[0] += ((uint32_t)inputLen << 3)) < ((uint32_t)inputLen << 3))
        context->count[1]++;
    context->count[1] += ((uint32_t)inputLen >> 29);
    partLen = 64 - index;
    if (inputLen >= partLen) {
        memcpy(&context->buffer[index], input, partLen);
        MD5Transform(context->state, context->buffer);
        for (i = partLen; i + 63 < inputLen; i += 64)
            MD5Transform(context->state, &input[i]);
        index = 0;
    } else {
        i = 0;
    }
    memcpy(&context->buffer[index], &input[i], inputLen - i);
}

static void MD5Final(uint8_t digest[16], MD5_CTX* context) {
    uint8_t bits[8];
    uint32_t index, padLen;
    static const uint8_t PADDING[64] = { 0x80 };
    bits[0] = context->count[0] & 0xFF;
    bits[1] = (context->count[0] >> 8) & 0xFF;
    bits[2] = (context->count[0] >> 16) & 0xFF;
    bits[3] = (context->count[0] >> 24) & 0xFF;
    bits[4] = context->count[1] & 0xFF;
    bits[5] = (context->count[1] >> 8) & 0xFF;
    bits[6] = (context->count[1] >> 16) & 0xFF;
    bits[7] = (context->count[1] >> 24) & 0xFF;
    index = (uint32_t)((context->count[0] >> 3) & 0x3f);
    padLen = (index < 56) ? (56 - index) : (120 - index);
    MD5Update(context, PADDING, padLen);
    MD5Update(context, bits, 8);
    memcpy(digest, context->state, 16);
    memset(context, 0, sizeof(*context));
}

#define F(x, y, z) (((x) & (y)) | ((~x) & (z)))
#define G(x, y, z) (((x) & (z)) | ((y) & (~z)))
#define H(x, y, z) ((x) ^ (y) ^ (z))
#define I(x, y, z) ((y) ^ ((x) | (~z)))
#define ROTATE_LEFT(x, n) (((x) << (n)) | ((x) >> (32-(n))))

#define FF(a, b, c, d, x, s, ac) { \
    (a) += F ((b), (c), (d)) + (x) + (uint32_t)(ac); \
    (a) = ROTATE_LEFT ((a), (s)); \
    (a) += (b); \
  }
#define GG(a, b, c, d, x, s, ac) { \
    (a) += G ((b), (c), (d)) + (x) + (uint32_t)(ac); \
    (a) = ROTATE_LEFT ((a), (s)); \
    (a) += (b); \
  }
#define HH(a, b, c, d, x, s, ac) { \
    (a) += H ((b), (c), (d)) + (x) + (uint32_t)(ac); \
    (a) = ROTATE_LEFT ((a), (s)); \
    (a) += (b); \
  }
#define II(a, b, c, d, x, s, ac) { \
    (a) += I ((b), (c), (d)) + (x) + (uint32_t)(ac); \
    (a) = ROTATE_LEFT ((a), (s)); \
    (a) += (b); \
  }

static void MD5Transform(uint32_t state[4], const uint8_t block[64]) {
    uint32_t a = state[0], b = state[1], c = state[2], d = state[3], x[16];
    for (int i = 0, j = 0; j < 64; i++, j += 4) {
        x[i] = ((uint32_t)block[j]) | (((uint32_t)block[j + 1]) << 8) |
               (((uint32_t)block[j + 2]) << 16) | (((uint32_t)block[j + 3]) << 24);
    }
    FF(a, b, c, d, x[0], 7, 0xd76aa478);
    FF(d, a, b, c, x[1], 12, 0xe8c7b756);
    FF(c, d, a, b, x[2], 17, 0x242070db);
    FF(b, c, d, a, x[3], 22, 0xc1bdceee);
    FF(a, b, c, d, x[4], 7, 0xf57c0faf);
    FF(d, a, b, c, x[5], 12, 0x4787c62a);
    FF(c, d, a, b, x[6], 17, 0xa8304613);
    FF(b, c, d, a, x[7], 22, 0xfd469501);
    FF(a, b, c, d, x[8], 7, 0x698098d8);
    FF(d, a, b, c, x[9], 12, 0x8b44f7af);
    FF(c, d, a, b, x[10], 17, 0xffff5bb1);
    FF(b, c, d, a, x[11], 22, 0x895cd7be);
    FF(a, b, c, d, x[12], 7, 0x6b901122);
    FF(d, a, b, c, x[13], 12, 0xfd987193);
    FF(c, d, a, b, x[14], 17, 0xa679438e);
    FF(b, c, d, a, x[15], 22, 0x49b40821);
    GG(a, b, c, d, x[1], 5, 0xf61e2562);
    GG(d, a, b, c, x[6], 9, 0xc040b340);
    GG(c, d, a, b, x[11], 14, 0x265e5a51);
    GG(b, c, d, a, x[0], 20, 0xe9b6c7aa);
    GG(a, b, c, d, x[5], 5, 0xd62f105d);
    GG(d, a, b, c, x[10], 9,  0x2441453);
    GG(c, d, a, b, x[15], 14, 0xd8a1e681);
    GG(b, c, d, a, x[4], 20, 0xe7d3fbc8);
    GG(a, b, c, d, x[9], 5, 0x21e1cde6);
    GG(d, a, b, c, x[14], 9, 0xc33707d6);
    GG(c, d, a, b, x[3], 14, 0xf4d50d87);
    GG(b, c, d, a, x[8], 20, 0x455a14ed);
    GG(a, b, c, d, x[13], 5, 0xa9e3e905);
    GG(d, a, b, c, x[2], 9, 0xfcefa3f8);
    GG(c, d, a, b, x[7], 14, 0x676f02d9);
    GG(b, c, d, a, x[12], 20, 0x8d2a4c8a);
    HH(a, b, c, d, x[5], 4, 0xfffa3942);
    HH(d, a, b, c, x[8], 11, 0x8771f681);
    HH(c, d, a, b, x[11], 16, 0x6d9d6122);
    HH(b, c, d, a, x[14], 23, 0xfde5380c);
    HH(a, b, c, d, x[1], 4, 0xa4beea44);
    HH(d, a, b, c, x[4], 11, 0x4bdecfa9);
    HH(c, d, a, b, x[7], 16, 0xf6bb4b60);
    HH(b, c, d, a, x[10], 23, 0xbebfbc70);
    HH(a, b, c, d, x[13], 4, 0x289b7ec6);
    HH(d, a, b, c, x[0], 11, 0xeaa127fa);
    HH(c, d, a, b, x[3], 16, 0xd4ef3085);
    HH(b, c, d, a, x[6], 23,  0x4881d05);
    HH(a, b, c, d, x[9], 4, 0xd9d4d039);
    HH(d, a, b, c, x[12], 11, 0xe6db99e5);
    HH(c, d, a, b, x[15], 16, 0x1fa27cf8);
    HH(b, c, d, a, x[2], 23, 0xc4ac5665);
    II(a, b, c, d, x[0], 6, 0xf4292244);
    II(d, a, b, c, x[7], 10, 0x432aff97);
    II(c, d, a, b, x[14], 15, 0xab9423a7);
    II(b, c, d, a, x[5], 21, 0xfc93a039);
    II(a, b, c, d, x[12], 6, 0x655b59c3);
    II(d, a, b, c, x[3], 10, 0x8f0ccc92);
    II(c, d, a, b, x[10], 15, 0xffeff47d);
    II(b, c, d, a, x[1], 21, 0x85845dd1);
    II(a, b, c, d, x[8], 6, 0x6fa87e4f);
    II(d, a, b, c, x[15], 10, 0xfe2ce6e0);
    II(c, d, a, b, x[6], 15, 0xa3014314);
    II(b, c, d, a, x[13], 21, 0x4e0811a1);
    II(a, b, c, d, x[4], 6, 0xf7537e82);
    II(d, a, b, c, x[11], 10, 0xbd3af235);
    II(c, d, a, b, x[2], 15, 0x2ad7d2bb);
    II(b, c, d, a, x[9], 21, 0xeb86d391);
    state[0] += a;
    state[1] += b;
    state[2] += c;
    state[3] += d;
    memset(x, 0, sizeof(x));
}

void* builtin_md5(const char* input) {
    if (!input) input = "";
    MD5_CTX context;
    uint8_t digest[16];
    MD5Init(&context);
    MD5Update(&context, (const uint8_t*)input, strlen(input));
    MD5Final(digest, &context);
    
    char* result = _alloc_ref(33, NULL);
    for (int i = 0; i < 16; i++) {
        sprintf(result + i * 2, "%02x", digest[i]);
    }
    result[32] = '\0';
    return result;
}

// --- SHA-256 ---
typedef struct {
    uint32_t state[8];
    uint64_t count;
    uint8_t buffer[64];
} SHA256_CTX;

static void SHA256Transform(uint32_t state[8], const uint8_t block[64]) {
    uint32_t w[64], a, b, c, d, e, f, g, h, t1, t2;
    static const uint32_t k[64] = {
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
        0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
        0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
        0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
        0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
        0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
        0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2
    };
    for (int i = 0; i < 16; i++) {
        w[i] = ((uint32_t)block[i*4] << 24) | ((uint32_t)block[i*4+1] << 16) |
               ((uint32_t)block[i*4+2] << 8) | ((uint32_t)block[i*4+3]);
    }
    for (int i = 16; i < 64; i++) {
        uint32_t s0 = (w[i-15] >> 7 | w[i-15] << 25) ^ (w[i-15] >> 18 | w[i-15] << 14) ^ (w[i-15] >> 3);
        uint32_t s1 = (w[i-2] >> 17 | w[i-2] << 15) ^ (w[i-2] >> 19 | w[i-2] << 13) ^ (w[i-2] >> 10);
        w[i] = w[i-16] + s0 + w[i-7] + s1;
    }
    a = state[0]; b = state[1]; c = state[2]; d = state[3];
    e = state[4]; f = state[5]; g = state[6]; h = state[7];
    for (int i = 0; i < 64; i++) {
        uint32_t s1 = (e >> 6 | e << 26) ^ (e >> 11 | e << 21) ^ (e >> 25 | e << 7);
        uint32_t ch = (e & f) ^ (~e & g);
        t1 = h + s1 + ch + k[i] + w[i];
        uint32_t s0 = (a >> 2 | a << 30) ^ (a >> 13 | a << 19) ^ (a >> 22 | a << 10);
        uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
        t2 = s0 + maj;
        h = g; g = f; f = e; e = d + t1;
        d = c; c = b; b = a; a = t1 + t2;
    }
    state[0] += a; state[1] += b; state[2] += c; state[3] += d;
    state[4] += e; state[5] += f; state[6] += g; state[7] += h;
}

static void SHA256Init(SHA256_CTX* ctx) {
    ctx->state[0] = 0x6a09e667;
    ctx->state[1] = 0xbb67ae85;
    ctx->state[2] = 0x3c6ef372;
    ctx->state[3] = 0xa54ff53a;
    ctx->state[4] = 0x510e527f;
    ctx->state[5] = 0x9b05688c;
    ctx->state[6] = 0x1f83d9ab;
    ctx->state[7] = 0x5be0cd19;
    ctx->count = 0;
}

static void SHA256Update(SHA256_CTX* ctx, const uint8_t* data, size_t len) {
    for (size_t i = 0; i < len; i++) {
        ctx->buffer[ctx->count % 64] = data[i];
        ctx->count++;
        if (ctx->count % 64 == 0) {
            SHA256Transform(ctx->state, ctx->buffer);
        }
    }
}

static void SHA256Final(uint8_t digest[32], SHA256_CTX* ctx) {
    uint64_t bitlen = ctx->count * 8;
    SHA256Update(ctx, (const uint8_t*)"\x80", 1);
    while (ctx->count % 64 != 56) {
        SHA256Update(ctx, (const uint8_t*)"\x00", 1);
    }
    uint8_t bits[8];
    for (int i = 0; i < 8; i++) {
        bits[i] = (bitlen >> (56 - i * 8)) & 0xFF;
    }
    SHA256Update(ctx, bits, 8);
    for (int i = 0; i < 8; i++) {
        digest[i*4] = (ctx->state[i] >> 24) & 0xFF;
        digest[i*4+1] = (ctx->state[i] >> 16) & 0xFF;
        digest[i*4+2] = (ctx->state[i] >> 8) & 0xFF;
        digest[i*4+3] = ctx->state[i] & 0xFF;
    }
}

void* builtin_sha256(const char* input) {
    if (!input) input = "";
    SHA256_CTX context;
    uint8_t digest[32];
    SHA256Init(&context);
    SHA256Update(&context, (const uint8_t*)input, strlen(input));
    SHA256Final(digest, &context);
    
    char* result = _alloc_ref(65, NULL);
    for (int i = 0; i < 32; i++) {
        sprintf(result + i * 2, "%02x", digest[i]);
    }
    result[64] = '\0';
    return result;
}

// --- Base64 ---
static const char b64_table[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

void* builtin_base64_encode(const char* input) {
    if (!input) input = "";
    int len = strlen(input);
    int out_len = 4 * ((len + 2) / 3);
    char* result = _alloc_ref(out_len + 1, NULL);
    
    int i, j;
    for (i = 0, j = 0; i < len;) {
        uint32_t octet_a = i < len ? (unsigned char)input[i++] : 0;
        uint32_t octet_b = i < len ? (unsigned char)input[i++] : 0;
        uint32_t octet_c = i < len ? (unsigned char)input[i++] : 0;
        uint32_t triple = (octet_a << 16) + (octet_b << 8) + octet_c;
        
        result[j++] = b64_table[(triple >> 18) & 0x3F];
        result[j++] = b64_table[(triple >> 12) & 0x3F];
        result[j++] = b64_table[(triple >> 6) & 0x3F];
        result[j++] = b64_table[triple & 0x3F];
    }
    
    int pad = (3 - (len % 3)) % 3;
    for (i = 0; i < pad; i++) {
        result[out_len - 1 - i] = '=';
    }
    result[out_len] = '\0';
    return result;
}

void* builtin_base64_decode(const char* input) {
    if (!input) input = "";
    int len = strlen(input);
    if (len % 4 != 0) return _alloc_ref(1, NULL);
    
    int out_len = (len / 4) * 3;
    if (len > 0 && input[len - 1] == '=') out_len--;
    if (len > 1 && input[len - 2] == '=') out_len--;
    
    char* result = _alloc_ref(out_len + 1, NULL);
    static char decoding_table[256];
    static int initialized = 0;
    if (!initialized) {
        for (int i = 0; i < 64; i++) decoding_table[(unsigned char)b64_table[i]] = i;
        initialized = 1;
    }
    
    int i, j;
    for (i = 0, j = 0; i < len;) {
        uint32_t sextet_a = input[i] == '=' ? 0 & i++ : decoding_table[(unsigned char)input[i++]];
        uint32_t sextet_b = input[i] == '=' ? 0 & i++ : decoding_table[(unsigned char)input[i++]];
        uint32_t sextet_c = input[i] == '=' ? 0 & i++ : decoding_table[(unsigned char)input[i++]];
        uint32_t sextet_d = input[i] == '=' ? 0 & i++ : decoding_table[(unsigned char)input[i++]];
        uint32_t triple = (sextet_a << 18) + (sextet_b << 12) + (sextet_c << 6) + sextet_d;
        
        if (j < out_len) result[j++] = (triple >> 16) & 0xFF;
        if (j < out_len) result[j++] = (triple >> 8) & 0xFF;
        if (j < out_len) result[j++] = triple & 0xFF;
    }
    result[out_len] = '\0';
    return result;
}

// =============================================================================
// 3. JSON Parser & Serializer
// =============================================================================

typedef enum {
    JSON_NULL,
    JSON_BOOL,
    JSON_NUMBER,
    JSON_STRING,
    JSON_ARRAY,
    JSON_OBJECT
} JSONType;

typedef struct JSONNode {
    JSONType type;
    double number_val;
    char* string_val;
    int bool_val;
    struct JSONNode** array_items;
    int array_count;
    char** object_keys;
    struct JSONNode** object_values;
    int object_count;
} JSONNode;

static void free_json_node(JSONNode* node) {
    if (!node) return;
    if (node->string_val) free(node->string_val);
    if (node->array_items) {
        for (int i = 0; i < node->array_count; i++) {
            free_json_node(node->array_items[i]);
        }
        free(node->array_items);
    }
    if (node->object_keys) {
        for (int i = 0; i < node->object_count; i++) {
            free(node->object_keys[i]);
            free_json_node(node->object_values[i]);
        }
        free(node->object_keys);
        free(node->object_values);
    }
    free(node);
}

static void free_json_node_destructor(void* ptr) {
    if (!ptr) return;
    JSONNode* node = *(JSONNode**)ptr;
    free_json_node(node);
}

static void skip_whitespace(const char** s) {
    while (**s == ' ' || **s == '\t' || **s == '\r' || **s == '\n') {
        (*s)++;
    }
}

static char* parse_string_raw(const char** s) {
    (*s)++; // skip quote
    const char* start = *s;
    while (**s && **s != '"') {
        if (**s == '\\') (*s)++;
        (*s)++;
    }
    int len = *s - start;
    char* str = (char*)malloc(len + 1);
    memcpy(str, start, len);
    str[len] = '\0';
    if (**s == '"') (*s)++;
    return str;
}

static JSONNode* parse_json_value(const char** s, int depth);

static JSONNode* parse_json_object(const char** s, int depth) {
    if (depth > 100) return NULL;
    (*s)++; // skip '{'
    JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
    node->type = JSON_OBJECT;
    skip_whitespace(s);
    if (**s == '}') {
        (*s)++;
        return node;
    }
    while (**s) {
        skip_whitespace(s);
        if (**s != '"') {
            free_json_node(node);
            return NULL;
        }
        char* key = parse_string_raw(s);
        skip_whitespace(s);
        if (**s != ':') {
            free(key);
            free_json_node(node);
            return NULL;
        }
        (*s)++; // skip ':'
        JSONNode* val = parse_json_value(s, depth + 1);
        if (!val) {
            free(key);
            free_json_node(node);
            return NULL;
        }
        
        node->object_keys = (char**)realloc(node->object_keys, sizeof(char*) * (node->object_count + 1));
        node->object_values = (JSONNode**)realloc(node->object_values, sizeof(JSONNode*) * (node->object_count + 1));
        node->object_keys[node->object_count] = key;
        node->object_values[node->object_count] = val;
        node->object_count++;
        
        skip_whitespace(s);
        if (**s == ',') {
            (*s)++;
        } else if (**s == '}') {
            (*s)++;
            break;
        } else {
            free_json_node(node);
            return NULL;
        }
    }
    return node;
}

static JSONNode* parse_json_array(const char** s, int depth) {
    if (depth > 100) return NULL;
    (*s)++; // skip '['
    JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
    node->type = JSON_ARRAY;
    skip_whitespace(s);
    if (**s == ']') {
        (*s)++;
        return node;
    }
    while (**s) {
        JSONNode* val = parse_json_value(s, depth + 1);
        if (!val) {
            free_json_node(node);
            return NULL;
        }
        node->array_items = (JSONNode**)realloc(node->array_items, sizeof(JSONNode*) * (node->array_count + 1));
        node->array_items[node->array_count] = val;
        node->array_count++;
        
        skip_whitespace(s);
        if (**s == ',') {
            (*s)++;
        } else if (**s == ']') {
            (*s)++;
            break;
        } else {
            free_json_node(node);
            return NULL;
        }
    }
    return node;
}

static JSONNode* parse_json_value(const char** s, int depth) {
    if (depth > 100) return NULL;
    skip_whitespace(s);
    if (**s == '{') return parse_json_object(s, depth);
    if (**s == '[') return parse_json_array(s, depth);
    if (**s == '"') {
        JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
        node->type = JSON_STRING;
        node->string_val = parse_string_raw(s);
        return node;
    }
    if (strncmp(*s, "true", 4) == 0) {
        (*s) += 4;
        JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
        node->type = JSON_BOOL;
        node->bool_val = 1;
        return node;
    }
    if (strncmp(*s, "false", 5) == 0) {
        (*s) += 5;
        JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
        node->type = JSON_BOOL;
        node->bool_val = 0;
        return node;
    }
    if (strncmp(*s, "null", 4) == 0) {
        (*s) += 4;
        JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
        node->type = JSON_NULL;
        return node;
    }
    // Number parsing
    char* end;
    double val = strtod(*s, &end);
    if (end != *s) {
        *s = end;
        JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
        node->type = JSON_NUMBER;
        node->number_val = val;
        return node;
    }
    return NULL;
}

void* builtin_json_parse(const char* text) {
    if (!text) text = "{}";
    const char* ptr = text;
    JSONNode* node = parse_json_value(&ptr, 0);
    if (!node) {
        node = (JSONNode*)calloc(1, sizeof(JSONNode));
        node->type = JSON_NULL;
    }
    void** ref = (void**)_alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
    *ref = node;
    return ref;
}

static void serialize_json_node(JSONNode* node, char** buf, int* cap, int* len) {
    if (!node) return;
    char temp[64];
    int temp_len = 0;
    
    switch (node->type) {
        case JSON_NULL:
            strcpy(temp, "null");
            temp_len = 4;
            break;
        case JSON_BOOL:
            strcpy(temp, node->bool_val ? "true" : "false");
            temp_len = node->bool_val ? 4 : 5;
            break;
        case JSON_NUMBER:
            sprintf(temp, "%g", node->number_val);
            temp_len = strlen(temp);
            break;
        case JSON_STRING:
            // Write string with quotes
            if (*len + (int)strlen(node->string_val) + 3 >= *cap) {
                *cap = (*cap + (int)strlen(node->string_val) + 3) * 2;
                *buf = realloc(*buf, *cap);
            }
            (*buf)[(*len)++] = '"';
            strcpy(*buf + *len, node->string_val);
            *len += strlen(node->string_val);
            (*buf)[(*len)++] = '"';
            return;
        case JSON_ARRAY:
            if (*len + 2 >= *cap) {
                *cap *= 2;
                *buf = realloc(*buf, *cap);
            }
            (*buf)[(*len)++] = '[';
            for (int i = 0; i < node->array_count; i++) {
                serialize_json_node(node->array_items[i], buf, cap, len);
                if (i < node->array_count - 1) {
                    if (*len + 2 >= *cap) { *cap *= 2; *buf = realloc(*buf, *cap); }
                    (*buf)[(*len)++] = ',';
                }
            }
            if (*len + 2 >= *cap) { *cap *= 2; *buf = realloc(*buf, *cap); }
            (*buf)[(*len)++] = ']';
            return;
        case JSON_OBJECT:
            if (*len + 2 >= *cap) {
                *cap *= 2;
                *buf = realloc(*buf, *cap);
            }
            (*buf)[(*len)++] = '{';
            for (int i = 0; i < node->object_count; i++) {
                int key_len = strlen(node->object_keys[i]);
                if (*len + key_len + 4 >= *cap) {
                    *cap = (*cap + key_len + 4) * 2;
                    *buf = realloc(*buf, *cap);
                }
                (*buf)[(*len)++] = '"';
                strcpy(*buf + *len, node->object_keys[i]);
                *len += key_len;
                (*buf)[(*len)++] = '"';
                (*buf)[(*len)++] = ':';
                serialize_json_node(node->object_values[i], buf, cap, len);
                if (i < node->object_count - 1) {
                    if (*len + 2 >= *cap) { *cap *= 2; *buf = realloc(*buf, *cap); }
                    (*buf)[(*len)++] = ',';
                }
            }
            if (*len + 2 >= *cap) { *cap *= 2; *buf = realloc(*buf, *cap); }
            (*buf)[(*len)++] = '}';
            return;
    }
    
    if (*len + temp_len >= *cap) {
        *cap = (*cap + temp_len) * 2;
        *buf = realloc(*buf, *cap);
    }
    strcpy(*buf + *len, temp);
    *len += temp_len;
}

void* builtin_json_write(void* json_ref) {
    if (!json_ref) return _alloc_ref(3, NULL);
    JSONNode* node = *(JSONNode**)json_ref;
    int cap = 256;
    int len = 0;
    char* buf = malloc(cap);
    serialize_json_node(node, &buf, &cap, &len);
    buf[len] = '\0';
    
    char* ret = _alloc_ref(len + 1, NULL);
    strcpy(ret, buf);
    free(buf);
    return ret;
}

double builtin_json_get_number(void* json_ref, const char* key) {
    if (!json_ref || !key) return 0.0;
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) return 0.0;
    for (int i = 0; i < node->object_count; i++) {
        if (strcmp(node->object_keys[i], key) == 0) {
            if (node->object_values[i]->type == JSON_NUMBER)
                return node->object_values[i]->number_val;
            return 0.0;
        }
    }
    return 0.0;
}

void* builtin_json_get_string(void* json_ref, const char* key) {
    if (!json_ref || !key) return _alloc_ref(1, NULL);
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) return _alloc_ref(1, NULL);
    for (int i = 0; i < node->object_count; i++) {
        if (strcmp(node->object_keys[i], key) == 0) {
            if (node->object_values[i]->type == JSON_STRING) {
                char* s = node->object_values[i]->string_val;
                char* ret = _alloc_ref(strlen(s) + 1, NULL);
                strcpy(ret, s);
                return ret;
            }
            return _alloc_ref(1, NULL);
        }
    }
    return _alloc_ref(1, NULL);
}

int64_t builtin_json_get_bool(void* json_ref, const char* key) {
    if (!json_ref || !key) return 0;
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) return 0;
    for (int i = 0; i < node->object_count; i++) {
        if (strcmp(node->object_keys[i], key) == 0) {
            if (node->object_values[i]->type == JSON_BOOL)
                return node->object_values[i]->bool_val;
            return 0;
        }
    }
    return 0;
}

void* builtin_json_get_object(void* json_ref, const char* key) {
    if (!json_ref || !key) return _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) {
        JSONNode* dummy = (JSONNode*)calloc(1, sizeof(JSONNode));
        dummy->type = JSON_NULL;
        void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
        *ref = dummy;
        return ref;
    }
    for (int i = 0; i < node->object_count; i++) {
        if (strcmp(node->object_keys[i], key) == 0) {
            // Keep a strong reference count on this JSON node sub-tree
            // Note: because the parent node might get freed, in a simple C AST
            // we should duplicate the node, or keep a ref count.
            // For safety and correctness, we will do a deep copy of the subnode:
            JSONNode* clone_node(JSONNode* src);
            JSONNode* sub_clone = clone_node(node->object_values[i]);
            void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
            *ref = sub_clone;
            return ref;
        }
    }
    JSONNode* dummy = (JSONNode*)calloc(1, sizeof(JSONNode));
    dummy->type = JSON_NULL;
    void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
    *ref = dummy;
    return ref;
}

void* builtin_json_get_list(void* json_ref, const char* key) {
    return builtin_json_get_object(json_ref, key); // Objects and lists share the same pointer handle type
}

double builtin_json_list_size(void* json_ref) {
    if (!json_ref) return 0;
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_ARRAY) return 0;
    return (double)node->array_count;
}

void* builtin_json_list_get(void* json_ref, double idx) {
    int index = (int)idx;
    if (!json_ref || index < 0) {
        JSONNode* dummy = (JSONNode*)calloc(1, sizeof(JSONNode));
        dummy->type = JSON_NULL;
        void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
        *ref = dummy;
        return ref;
    }
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_ARRAY || index >= node->array_count) {
        JSONNode* dummy = (JSONNode*)calloc(1, sizeof(JSONNode));
        dummy->type = JSON_NULL;
        void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
        *ref = dummy;
        return ref;
    }
    JSONNode* clone_node(JSONNode* src);
    JSONNode* sub_clone = clone_node(node->array_items[index]);
    void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
    *ref = sub_clone;
    return ref;
}

JSONNode* clone_node(JSONNode* src) {
    if (!src) return NULL;
    JSONNode* dst = (JSONNode*)calloc(1, sizeof(JSONNode));
    dst->type = src->type;
    dst->number_val = src->number_val;
    dst->bool_val = src->bool_val;
    if (src->string_val) {
        dst->string_val = strdup(src->string_val);
    }
    if (src->array_items) {
        dst->array_items = (JSONNode**)malloc(sizeof(JSONNode*) * src->array_count);
        dst->array_count = src->array_count;
        for (int i = 0; i < src->array_count; i++) {
            dst->array_items[i] = clone_node(src->array_items[i]);
        }
    }
    if (src->object_keys) {
        dst->object_keys = (char**)malloc(sizeof(char*) * src->object_count);
        dst->object_values = (JSONNode**)malloc(sizeof(JSONNode*) * src->object_count);
        dst->object_count = src->object_count;
        for (int i = 0; i < src->object_count; i++) {
            dst->object_keys[i] = strdup(src->object_keys[i]);
            dst->object_values[i] = clone_node(src->object_values[i]);
        }
    }
    return dst;
}

void* builtin_json_create() {
    JSONNode* node = (JSONNode*)calloc(1, sizeof(JSONNode));
    node->type = JSON_OBJECT;
    void** ref = _alloc_ref(sizeof(JSONNode*), free_json_node_destructor);
    *ref = node;
    return ref;
}

void builtin_json_add_number(void* json_ref, const char* key, double val) {
    if (!json_ref || !key) return;
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) return;
    JSONNode* sub = (JSONNode*)calloc(1, sizeof(JSONNode));
    sub->type = JSON_NUMBER;
    sub->number_val = val;
    
    node->object_keys = (char**)realloc(node->object_keys, sizeof(char*) * (node->object_count + 1));
    node->object_values = (JSONNode**)realloc(node->object_values, sizeof(JSONNode*) * (node->object_count + 1));
    node->object_keys[node->object_count] = strdup(key);
    node->object_values[node->object_count] = sub;
    node->object_count++;
}

void builtin_json_add_string(void* json_ref, const char* key, const char* val) {
    if (!json_ref || !key) return;
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) return;
    JSONNode* sub = (JSONNode*)calloc(1, sizeof(JSONNode));
    sub->type = JSON_STRING;
    sub->string_val = strdup(val ? val : "");
    
    node->object_keys = (char**)realloc(node->object_keys, sizeof(char*) * (node->object_count + 1));
    node->object_values = (JSONNode**)realloc(node->object_values, sizeof(JSONNode*) * (node->object_count + 1));
    node->object_keys[node->object_count] = strdup(key);
    node->object_values[node->object_count] = sub;
    node->object_count++;
}

void builtin_json_add_bool(void* json_ref, const char* key, int64_t val) {
    if (!json_ref || !key) return;
    JSONNode* node = *(JSONNode**)json_ref;
    if (node->type != JSON_OBJECT) return;
    JSONNode* sub = (JSONNode*)calloc(1, sizeof(JSONNode));
    sub->type = JSON_BOOL;
    sub->bool_val = val ? 1 : 0;
    
    node->object_keys = (char**)realloc(node->object_keys, sizeof(char*) * (node->object_count + 1));
    node->object_values = (JSONNode**)realloc(node->object_values, sizeof(JSONNode*) * (node->object_count + 1));
    node->object_keys[node->object_count] = strdup(key);
    node->object_values[node->object_count] = sub;
    node->object_count++;
}

void builtin_json_add_object(void* json_ref, const char* key, void* sub_ref) {
    if (!json_ref || !key || !sub_ref) return;
    JSONNode* node = *(JSONNode**)json_ref;
    JSONNode* sub = *(JSONNode**)sub_ref;
    if (node->type != JSON_OBJECT) return;
    
    node->object_keys = (char**)realloc(node->object_keys, sizeof(char*) * (node->object_count + 1));
    node->object_values = (JSONNode**)realloc(node->object_values, sizeof(JSONNode*) * (node->object_count + 1));
    node->object_keys[node->object_count] = strdup(key);
    node->object_values[node->object_count] = clone_node(sub);
    node->object_count++;
}

void builtin_json_add_list(void* json_ref, const char* key, void* sub_ref) {
    builtin_json_add_object(json_ref, key, sub_ref);
}

// =============================================================================
// 4. Threading & Concurrency Support
// =============================================================================

int64_t _thread_spawn(void* (*func)(void*), void* arg) {
#if defined(_WIN32) || defined(_WIN64)
    HANDLE h = CreateThread(NULL, 0, (LPTHREAD_START_ROUTINE)func, arg, 0, NULL);
    return (int64_t)h;
#else
    pthread_t t;
    if (pthread_create(&t, NULL, func, arg) != 0) {
        return 0;
    }
    return (int64_t)t;
#endif
}

int64_t builtin_thread_join(int64_t handle) {
#if defined(_WIN32) || defined(_WIN64)
    HANDLE h = (HANDLE)handle;
    if (h) {
        WaitForSingleObject(h, INFINITE);
        CloseHandle(h);
    }
    return 0;
#else
    pthread_t t = (pthread_t)handle;
    pthread_join(t, NULL);
    return 0;
#endif
}

void _runtime_null_pointer_error_c() {
    printf("Қате: Нөлдік сілтеме (Null Pointer Exception)\n");
    exit(1);
}

void _runtime_divide_by_zero_error_c() {
    printf("Қате: Нөлге бөлу (Division by zero)\n");
    exit(1);
}

void _runtime_array_bounds_error_c() {
    printf("Қате: Индекс шектен шықты (Array Index Out of Bounds)\n");
    exit(1);
}

void* builtin_input() {
    char buf[4096];
    memset(buf, 0, sizeof(buf));
    if (fgets(buf, sizeof(buf), stdin)) {
        size_t len = strlen(buf);
        if (len > 0 && buf[len - 1] == '\n') {
            buf[len - 1] = '\0';
            len--;
        }
        if (len > 0 && buf[len - 1] == '\r') {
            buf[len - 1] = '\0';
            len--;
        }
        void* ref = _alloc_ref(len + 1, NULL);
        strcpy((char*)ref, buf);
        return ref;
    }
    void* ref = _alloc_ref(1, NULL);
    ((char*)ref)[0] = '\0';
    return ref;
}

// --- Time and Sleep Support ---
#include <time.h>
#if defined(_WIN32) || defined(_WIN64)
#include <sys/timeb.h>
#else
#include <sys/time.h>
#endif

double builtin_time_seconds() {
#if defined(_WIN32) || defined(_WIN64)
    struct _timeb tb;
    _ftime(&tb);
    return (double)tb.time + (double)tb.millitm / 1000.0;
#else
    struct timeval tv;
    gettimeofday(&tv, NULL);
    return (double)tv.tv_sec + (double)tv.tv_usec / 1000000.0;
#endif
}

void* builtin_time_str(const char* format) {
    if (!format || strlen(format) == 0) format = "%Y-%m-%d %H:%M:%S";
    time_t rawtime;
    struct tm* timeinfo;
    char buffer[256];
    time(&rawtime);
    timeinfo = localtime(&rawtime);
    strftime(buffer, sizeof(buffer), format, timeinfo);
    
    char* res = _alloc_ref(strlen(buffer) + 1, NULL);
    strcpy(res, buffer);
    return res;
}

void builtin_sleep_seconds(double seconds) {
    if (seconds < 0) return;
#if defined(_WIN32) || defined(_WIN64)
    Sleep((DWORD)(seconds * 1000.0));
#else
    usleep((useconds_t)(seconds * 1000000.0));
#endif
}

void* builtin_utf8_char_at(const char* str, double idx) {
    if (!str) return _alloc_ref(1, NULL);
    int target_char_idx = (int)idx;
    if (target_char_idx < 0) return _alloc_ref(1, NULL);
    
    int current_char_idx = 0;
    const uint8_t* p = (const uint8_t*)str;
    while (*p && current_char_idx < target_char_idx) {
        uint8_t c = *p;
        if (c < 0x80) p += 1;
        else if ((c & 0xE0) == 0xC0) p += 2;
        else if ((c & 0xF0) == 0xE0) p += 3;
        else if ((c & 0xF8) == 0xF0) p += 4;
        else p += 1;
        current_char_idx++;
    }
    
    if (*p == '\0') {
        return _alloc_ref(1, NULL);
    }
    
    const uint8_t* start = p;
    uint8_t c = *p;
    int len = 0;
    if (c < 0x80) len = 1;
    else if ((c & 0xE0) == 0xC0) len = 2;
    else if ((c & 0xF0) == 0xE0) len = 3;
    else if ((c & 0xF8) == 0xF0) len = 4;
    else len = 1;
    
    int real_len = 0;
    while (real_len < len && start[real_len]) {
        real_len++;
    }
    
    char* res = _alloc_ref(real_len + 1, NULL);
    memcpy(res, start, real_len);
    res[real_len] = '\0';
    return res;
}

#include <ctype.h>
#include <math.h>

// =============================================================================
// Basic math wrappers (were called by Kazakh names directly)
// =============================================================================
double builtin_sqrt(double x)            { return sqrt(x); }
double builtin_pow(double base, double exp) { return pow(base, exp); }
double builtin_sin(double x)             { return sin(x); }
double builtin_cos(double x)             { return cos(x); }
double builtin_rand_float()              { return (double)rand(); }

// =============================================================================
// String utility functions
// =============================================================================

// мәтін_іздеу(str, sub) -> index (-1 if not found) as double
double builtin_str_index_of(const char* str, const char* sub) {
    if (!str || !sub) return -1.0;
    const char* found = strstr(str, sub);
    if (!found) return -1.0;
    return (double)(found - str);
}

// мәтін_басталады(str, prefix) -> 1 or 0 as double
double builtin_str_starts_with(const char* str, const char* prefix) {
    if (!str || !prefix) return 0.0;
    size_t plen = strlen(prefix);
    if (strlen(str) < plen) return 0.0;
    return strncmp(str, prefix, plen) == 0 ? 1.0 : 0.0;
}

// мәтін_аяқталады(str, suffix) -> 1 or 0 as double
double builtin_str_ends_with(const char* str, const char* suffix) {
    if (!str || !suffix) return 0.0;
    size_t slen = strlen(str);
    size_t suflen = strlen(suffix);
    if (slen < suflen) return 0.0;
    return strcmp(str + slen - suflen, suffix) == 0 ? 1.0 : 0.0;
}

// мәтін_бөлу(str, sep, index) -> the index-th part when str is split by sep
void* builtin_str_split_get(const char* str, const char* sep, double idx_d) {
    if (!str || !sep) return _alloc_ref(1, NULL);
    int target_idx = (int)idx_d;
    int sep_len = (int)strlen(sep);
    if (sep_len == 0) {
        int slen = (int)strlen(str);
        char* copy = _alloc_ref(slen + 1, NULL);
        strcpy(copy, str);
        return copy;
    }
    const char* p = str;
    int part = 0;
    while (1) {
        const char* found = strstr(p, sep);
        if (part == target_idx) {
            int len = found ? (int)(found - p) : (int)strlen(p);
            char* res = _alloc_ref(len + 1, NULL);
            memcpy(res, p, len);
            res[len] = '\0';
            return res;
        }
        if (!found) break;
        p = found + sep_len;
        part++;
    }
    return _alloc_ref(1, NULL);
}

// мәтін_бөлу_саны(str, sep) -> number of parts as double
double builtin_str_split_count(const char* str, const char* sep) {
    if (!str || !sep) return 0.0;
    int sep_len = (int)strlen(sep);
    if (sep_len == 0) return 1.0;
    int count = 1;
    const char* p = str;
    while ((p = strstr(p, sep)) != NULL) { count++; p += sep_len; }
    return (double)count;
}

// мәтін_ауыстыру(str, old, new) -> new string with replacements
void* builtin_str_replace(const char* str, const char* old_sub, const char* new_sub) {
    if (!str) { char* e = _alloc_ref(1, NULL); e[0]='\0'; return e; }
    if (!old_sub || strlen(old_sub) == 0) {
        int slen = (int)strlen(str);
        char* copy = _alloc_ref(slen + 1, NULL);
        strcpy(copy, str);
        return copy;
    }
    if (!new_sub) new_sub = "";
    size_t old_len = strlen(old_sub);
    size_t new_len = strlen(new_sub);
    int count = 0;
    const char* p = str;
    while ((p = strstr(p, old_sub)) != NULL) { count++; p += old_len; }
    size_t result_len = strlen(str) + count * ((int)new_len - (int)old_len);
    char* res = _alloc_ref((int)result_len + 1, NULL);
    char* w = res;
    p = str;
    const char* found;
    while ((found = strstr(p, old_sub)) != NULL) {
        size_t before = found - p;
        memcpy(w, p, before); w += before;
        memcpy(w, new_sub, new_len); w += new_len;
        p = found + old_len;
    }
    strcpy(w, p);
    return res;
}

// мәтін_кіші(str) -> lowercase string
void* builtin_str_lower(const char* str) {
    if (!str) return _alloc_ref(1, NULL);
    int len = (int)strlen(str);
    char* res = _alloc_ref(len + 1, NULL);
    for (int i = 0; i < len; i++) res[i] = (char)tolower((unsigned char)str[i]);
    res[len] = '\0';
    return res;
}

// мәтін_жоғары(str) -> uppercase string
void* builtin_str_upper(const char* str) {
    if (!str) return _alloc_ref(1, NULL);
    int len = (int)strlen(str);
    char* res = _alloc_ref(len + 1, NULL);
    for (int i = 0; i < len; i++) res[i] = (char)toupper((unsigned char)str[i]);
    res[len] = '\0';
    return res;
}

// мәтін_кесу(str, start, end) -> substring [start, end)
void* builtin_str_slice(const char* str, double start_d, double end_d) {
    if (!str) return _alloc_ref(1, NULL);
    int len = (int)strlen(str);
    int start = (int)start_d; if (start < 0) start = 0;
    int end   = (int)end_d;   if (end > len) end = len;
    if (start >= end) return _alloc_ref(1, NULL);
    int sl = end - start;
    char* res = _alloc_ref(sl + 1, NULL);
    memcpy(res, str + start, sl);
    res[sl] = '\0';
    return res;
}

// мәтін_қысқарту(str) -> trimmed string
void* builtin_str_trim(const char* str) {
    if (!str) return _alloc_ref(1, NULL);
    while (*str == ' ' || *str == '\t' || *str == '\n' || *str == '\r') str++;
    int len = (int)strlen(str);
    while (len > 0 && (str[len-1]==' '||str[len-1]=='\t'||str[len-1]=='\n'||str[len-1]=='\r')) len--;
    char* res = _alloc_ref(len + 1, NULL);
    memcpy(res, str, len); res[len] = '\0';
    return res;
}

// =============================================================================
// Extended math functions
// =============================================================================

double builtin_math_abs(double x)          { return fabs(x); }
double builtin_math_floor(double x)        { return floor(x); }
double builtin_math_ceil(double x)         { return ceil(x); }
double builtin_math_log(double x)          { return log(x); }
double builtin_math_log2(double x)         { return log2(x); }
double builtin_math_log10(double x)        { return log10(x); }
double builtin_math_min(double a, double b){ return a < b ? a : b; }
double builtin_math_max(double a, double b){ return a > b ? a : b; }
double builtin_math_round(double x)        { return round(x); }
double builtin_math_tan(double x)          { return tan(x); }
double builtin_math_atan(double x)         { return atan(x); }
double builtin_math_atan2(double y, double x){ return atan2(y, x); }
double builtin_math_pi()                   { return 3.14159265358979323846; }
double builtin_math_exp(double x)          { return exp(x); }

// =============================================================================
// Map / Dictionary  (string key -> number or string value)
// =============================================================================

#define BTQ_MAP_BUCKETS 256

typedef enum { BTQ_VAL_NUMBER = 0, BTQ_VAL_STRING = 1 } BtqValType;

typedef struct BtqMapEntry {
    char* key;
    BtqValType val_type;
    double     num_val;
    char*      str_val;
    struct BtqMapEntry* next;
} BtqMapEntry;

typedef struct {
    BtqMapEntry* buckets[BTQ_MAP_BUCKETS];
    int64_t size;
} BtqMap;

static uint32_t btq_map_hash(const char* key) {
    uint32_t h = 2166136261U;
    while (*key) { h ^= (uint8_t)(*key++); h *= 16777619U; }
    return h % BTQ_MAP_BUCKETS;
}

void* builtin_map_create() {
    BtqMap* m = (BtqMap*)calloc(1, sizeof(BtqMap));
    return m;
}

void builtin_map_set_number(void* mp, const char* key, double val) {
    if (!mp || !key) return;
    BtqMap* m = (BtqMap*)mp;
    uint32_t idx = btq_map_hash(key);
    for (BtqMapEntry* e = m->buckets[idx]; e; e = e->next) {
        if (strcmp(e->key, key) == 0) {
            e->val_type = BTQ_VAL_NUMBER; e->num_val = val;
            if (e->str_val) { free(e->str_val); e->str_val = NULL; }
            return;
        }
    }
    BtqMapEntry* ne = (BtqMapEntry*)calloc(1, sizeof(BtqMapEntry));
    ne->key = strdup(key); ne->val_type = BTQ_VAL_NUMBER; ne->num_val = val;
    ne->next = m->buckets[idx]; m->buckets[idx] = ne; m->size++;
}

void builtin_map_set_string(void* mp, const char* key, const char* val) {
    if (!mp || !key) return;
    BtqMap* m = (BtqMap*)mp;
    uint32_t idx = btq_map_hash(key);
    for (BtqMapEntry* e = m->buckets[idx]; e; e = e->next) {
        if (strcmp(e->key, key) == 0) {
            e->val_type = BTQ_VAL_STRING;
            if (e->str_val) free(e->str_val);
            e->str_val = strdup(val ? val : ""); return;
        }
    }
    BtqMapEntry* ne = (BtqMapEntry*)calloc(1, sizeof(BtqMapEntry));
    ne->key = strdup(key); ne->val_type = BTQ_VAL_STRING; ne->str_val = strdup(val ? val : "");
    ne->next = m->buckets[idx]; m->buckets[idx] = ne; m->size++;
}

double builtin_map_get_number(void* mp, const char* key) {
    if (!mp || !key) return 0.0;
    BtqMap* m = (BtqMap*)mp;
    uint32_t idx = btq_map_hash(key);
    for (BtqMapEntry* e = m->buckets[idx]; e; e = e->next)
        if (strcmp(e->key, key) == 0)
            return e->val_type == BTQ_VAL_NUMBER ? e->num_val : 0.0;
    return 0.0;
}

void* builtin_map_get_string(void* mp, const char* key) {
    if (!mp || !key) return _alloc_ref(1, NULL);
    BtqMap* m = (BtqMap*)mp;
    uint32_t idx = btq_map_hash(key);
    for (BtqMapEntry* e = m->buckets[idx]; e; e = e->next) {
        if (strcmp(e->key, key) == 0 && e->val_type == BTQ_VAL_STRING && e->str_val) {
            int len = (int)strlen(e->str_val);
            char* res = _alloc_ref(len + 1, NULL);
            memcpy(res, e->str_val, len + 1); return res;
        }
    }
    return _alloc_ref(1, NULL);
}

double builtin_map_has_key(void* mp, const char* key) {
    if (!mp || !key) return 0.0;
    BtqMap* m = (BtqMap*)mp;
    uint32_t idx = btq_map_hash(key);
    for (BtqMapEntry* e = m->buckets[idx]; e; e = e->next)
        if (strcmp(e->key, key) == 0) return 1.0;
    return 0.0;
}

void builtin_map_delete_key(void* mp, const char* key) {
    if (!mp || !key) return;
    BtqMap* m = (BtqMap*)mp;
    uint32_t idx = btq_map_hash(key);
    BtqMapEntry* e = m->buckets[idx]; BtqMapEntry* prev = NULL;
    while (e) {
        if (strcmp(e->key, key) == 0) {
            if (prev) prev->next = e->next; else m->buckets[idx] = e->next;
            free(e->key); if (e->str_val) free(e->str_val); free(e); m->size--; return;
        }
        prev = e; e = e->next;
    }
}

double builtin_map_size(void* mp) {
    if (!mp) return 0.0;
    return (double)((BtqMap*)mp)->size;
}

// =============================================================================
// Mutex synchronization support
// =============================================================================
void* builtin_mutex_create() {
#if defined(_WIN32) || defined(_WIN64)
    CRITICAL_SECTION* cs = (CRITICAL_SECTION*)malloc(sizeof(CRITICAL_SECTION));
    InitializeCriticalSection(cs);
    return cs;
#else
    pthread_mutex_t* mutex = (pthread_mutex_t*)malloc(sizeof(pthread_mutex_t));
    pthread_mutex_init(mutex, NULL);
    return mutex;
#endif
}

void builtin_mutex_lock(void* mutex) {
    if (!mutex) return;
#if defined(_WIN32) || defined(_WIN64)
    EnterCriticalSection((CRITICAL_SECTION*)mutex);
#else
    pthread_mutex_lock((pthread_mutex_t*)mutex);
#endif
}

void builtin_mutex_unlock(void* mutex) {
    if (!mutex) return;
#if defined(_WIN32) || defined(_WIN64)
    LeaveCriticalSection((CRITICAL_SECTION*)mutex);
#else
    pthread_mutex_unlock((pthread_mutex_t*)mutex);
#endif
}

void builtin_mutex_destroy(void* mutex) {
    if (!mutex) return;
#if defined(_WIN32) || defined(_WIN64)
    DeleteCriticalSection((CRITICAL_SECTION*)mutex);
#else
    pthread_mutex_destroy((pthread_mutex_t*)mutex);
#endif
    free(mutex);
}

// =============================================================================
// 5. Result (Error Handling) Support
// =============================================================================

typedef struct {
    int64_t is_error;
    union {
        double num;
        void* ptr;
    } value;
    const char* error_msg;
} ResultObject;

void* builtin_result_ok_num(double val) {
    ResultObject* res = (ResultObject*)malloc(sizeof(ResultObject));
    if (!res) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    res->is_error = 0;
    res->value.num = val;
    res->error_msg = NULL;
    return res;
}

void* builtin_result_ok_ptr(void* ptr) {
    ResultObject* res = (ResultObject*)malloc(sizeof(ResultObject));
    if (!res) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    res->is_error = 0;
    res->value.ptr = ptr;
    res->error_msg = NULL;
    return res;
}

void* builtin_result_err(const char* msg) {
    ResultObject* res = (ResultObject*)malloc(sizeof(ResultObject));
    if (!res) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    res->is_error = 1;
    res->value.ptr = NULL;
    res->error_msg = msg ? strdup(msg) : "Белгісіз қате";
    return res;
}

int64_t builtin_result_is_error(void* res) {
    if (!res) return 0;
    return ((ResultObject*)res)->is_error;
}

double builtin_result_get_num(void* res) {
    if (!res) return 0.0;
    return ((ResultObject*)res)->value.num;
}

void* builtin_result_get_ptr(void* res) {
    if (!res) return NULL;
    return ((ResultObject*)res)->value.ptr;
}

const char* builtin_result_get_err(void* res) {
    if (!res) return "";
    return ((ResultObject*)res)->error_msg;
}

// ── StringBuilder implementation ───────────────────────────────────────────
typedef struct {
    char* buf;
    int64_t cap;
    int64_t len;
} StringBuilder;

void* builtin_string_builder_create() {
    StringBuilder* sb = (StringBuilder*)malloc(sizeof(StringBuilder));
    if (!sb) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    sb->cap = 32;
    sb->len = 0;
    sb->buf = (char*)malloc(sb->cap);
    if (!sb->buf) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    sb->buf[0] = '\0';
    return sb;
}

void builtin_string_builder_append_string(void* sb_ref, const char* str) {
    if (!sb_ref || !str) return;
    StringBuilder* sb = (StringBuilder*)sb_ref;
    int64_t str_len = strlen(str);
    if (sb->len + str_len >= sb->cap) {
        sb->cap = (sb->len + str_len) * 2;
        sb->buf = (char*)realloc(sb->buf, sb->cap);
        if (!sb->buf) {
            fprintf(stderr, "Қате: жадты қайта бөлу сәтсіз аяқталды\n");
            exit(1);
        }
    }
    strcpy(sb->buf + sb->len, str);
    sb->len += str_len;
}

void builtin_string_builder_append_number(void* sb_ref, double num) {
    if (!sb_ref) return;
    char tmp[64];
    if (num == (int64_t)num) {
        sprintf(tmp, "%lld", (long long)num);
    } else {
        sprintf(tmp, "%g", num);
    }
    builtin_string_builder_append_string(sb_ref, tmp);
}

void builtin_string_builder_append_char(void* sb_ref, double code) {
    if (!sb_ref) return;
    StringBuilder* sb = (StringBuilder*)sb_ref;
    char ch = (char)code;
    if (sb->len + 1 >= sb->cap) {
        sb->cap *= 2;
        sb->buf = (char*)realloc(sb->buf, sb->cap);
        if (!sb->buf) {
            fprintf(stderr, "Қате: жадты қайта бөлу сәтсіз аяқталды\n");
            exit(1);
        }
    }
    sb->buf[sb->len] = ch;
    sb->len++;
    sb->buf[sb->len] = '\0';
}

void* builtin_string_builder_to_string(void* sb_ref) {
    if (!sb_ref) return strdup("");
    StringBuilder* sb = (StringBuilder*)sb_ref;
    return strdup(sb->buf);
}

void builtin_string_builder_destroy(void* sb_ref) {
    if (!sb_ref) return;
    StringBuilder* sb = (StringBuilder*)sb_ref;
    if (sb->buf) free(sb->buf);
    free(sb);
}

double builtin_utf8_str_len(const char* str) {
    if (!str) return 0.0;
    double count = 0;
    const uint8_t* p = (const uint8_t*)str;
    while (*p) {
        uint8_t c = *p;
        if (c < 0x80) p += 1;
        else if ((c & 0xE0) == 0xC0) p += 2;
        else if ((c & 0xF0) == 0xE0) p += 3;
        else if ((c & 0xF8) == 0xF0) p += 4;
        else p += 1;
        count += 1.0;
    }
    return count;
}

void* builtin_exec_output(const char* command) {
    if (!command) return _alloc_ref(1, NULL);
    FILE* fp;
#if defined(_WIN32) || defined(_WIN64)
    fp = _popen(command, "r");
#else
    fp = popen(command, "r");
#endif
    if (fp == NULL) {
        return _alloc_ref(1, NULL);
    }
    
    char path[1024];
    int cap = 1024;
    int len = 0;
    char* result = (char*)malloc(cap);
    if (!result) {
        fprintf(stderr, "Қате: жад бөлу сәтсіз аяқталды\n");
        exit(1);
    }
    result[0] = '\0';
    
    while (fgets(path, sizeof(path), fp) != NULL) {
        int bytes = strlen(path);
        if (len + bytes >= cap) {
            cap = (len + bytes) * 2;
            result = (char*)realloc(result, cap);
            if (!result) {
                fprintf(stderr, "Қате: жадты қайта бөлу сәтсіз аяқталды\n");
                exit(1);
            }
        }
        strcpy(result + len, path);
        len += bytes;
    }
    
#if defined(_WIN32) || defined(_WIN64)
    _pclose(fp);
#else
    pclose(fp);
#endif
    
    void* ref = _alloc_ref(len + 1, NULL);
    strcpy((char*)ref, result);
    free(result);
    return ref;
}

static int global_argc = 0;
static char** global_argv = NULL;

void builtin_init_args(int argc, char** argv) {
    global_argc = argc;
    global_argv = argv;
}

double builtin_args_count() {
    return (double)global_argc;
}

void* builtin_arg_get(double idx) {
    int i = (int)idx;
    if (i < 0 || i >= global_argc || !global_argv) {
        return _alloc_ref(1, NULL);
    }
    size_t len = strlen(global_argv[i]);
    char* ref = _alloc_ref(len + 1, NULL);
    strcpy(ref, global_argv[i]);
    return ref;
}

void* builtin_num_to_str(double x) {
    char buf[64];
    if (x == (int64_t)x) {
        sprintf(buf, "%lld", (long long)x);
    } else {
        sprintf(buf, "%g", x);
    }
    char* ref = _alloc_ref(strlen(buf) + 1, NULL);
    strcpy(ref, buf);
    return ref;
}

double builtin_str_to_num(const char* str) {
    if (!str) return 0.0;
    return strtod(str, NULL);
}

double builtin_system(const char* command) {
    if (!command) return -1.0;
    return (double)system(command);
}

double builtin_file_delete(const char* path) {
    if (!path) return -1.0;
    return (double)remove(path);
}

double builtin_file_exists(const char* path) {
    if (!path) return 0.0;
    FILE* f = fopen(path, "r");
    if (f) {
        fclose(f);
        return 1.0;
    }
    return 0.0;
}

