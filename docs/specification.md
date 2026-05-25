# Butaq Тілінің Техникалық Спецификациясы (Technical Specification)

Бұл құжат Butaq бағдарламалау тілінің синтаксисін, семантикасын, типтер жүйесін және орындалу уақытының (runtime) сипаттамаларын қамтиды.

---

## 1. Синтаксис және EBNF Грамматикасы

Butaq тілі қатаң **SOV (Subject-Object-Verb / Бастауыш-Баяндауыш-Пысықтауыш немесе Пысықтауыш соңында)** құрылымын қолданады. Бұл дегеніміз, кез келген әрекет немесе функция шақыруы сөйлемнің соңында орналасады.

### Негізгі ережелер:
```ebnf
Program         ::= Statement* EOF
Statement       ::= VarDecl | Assignment | IfStatement | WhileStatement | ReturnStatement | PrintStatement | ThreadStatement | ExpressionStatement
VarDecl         ::= Identifier Expression "болсын"
Assignment      ::= (Identifier | MemberAccess) Expression "болсын"
PrintStatement  ::= Expression "жазу"
ReturnStatement ::= Expression "қайтару"
IfStatement     ::= "егер" Expression Block ("әйтпесе" Block)?
WhileStatement  ::= "әзірше" Expression Block
Block           ::= "{" Statement* "}"

Expression      ::= LogicOr
LogicOr         ::= LogicAnd ("немесе" LogicAnd)*
LogicAnd        ::= Equality ("және" Equality)*
Equality        ::= Comparison (("тең" | "тең_емес") Comparison)*
Comparison      ::= Additive (("үлкен" | "кіші" | "үлкен_тең" | "кіші_тең") Additive)*
Additive        ::= Multiplicative (("қосу" | "алу") Multiplicative)*
Multiplicative  ::= Primary (("көбейту" | "бөлу") Primary)*

Primary         ::= Number | IntLiteral | String | Boolean | Identifier | MemberAccess | ArrayLiteral | ArrayAccess | FnCall | "(" Expression ")"
```

---

## 2. Типтер Жүйесі (Type System)

Butaq тілі — статикалық типтелген тіл. Барлық типтер компиляция кезеңінде тексеріледі.

| Butaq типі | С сәйкестігі | Сипаттамасы |
| :--- | :--- | :--- |
| `БҮТІН` | `int64_t` | 64-бит таңбалы бүтін сан |
| `САН` | `double` | 64-бит қос дәлдікті өзгермелі нүктелі сан |
| `МӘТІН` | `char*` | UTF-8 кодталған жол |
| `АҚИҚАТ` | `bool` / `i1` | Логикалық мән (`ақиқат` немесе `жалған`) |
| `ЖСОН` | `void*` | Жүйелік JSON нысаны немесе тізімі |
| `ҚҰРЫЛЫМ` | `struct*` | Пайдаланушы анықтаған нысан типі |

---

## 3. Жадты Басқару (Automatic Reference Counting - ARC)

Butaq жадты басқару үшін қосымша қоқыс жинағышсыз (Garbage Collector) **ARC (Automatic Reference Counting)** механизмін қолданады.

### RefHeader Құрылымы:
Әрбір динамикалық нысан кучада (heap) келесі тақырыптамамен (header) сақталады:
```c
typedef struct {
    int64_t strong_count;        // Күшті сілтемелер саны
    void (*destructor)(void*);   // Нысан жойылғанда шақырылатын деструктор функциясы
} RefHeader;
```

### Әлсіз сілтемелер (`әлсіз` / Weak References):
Сілтемелер циклін (cyclic references) және жадтың жылыстауын (memory leaks) болдырмау үшін Butaq **Zeroing Weak References** (нөлденетін әлсіз сілтемелер) жүйесін қолданады:
1. Әлсіз сілтеме нысанның күшті сілтемелер санын арттырмайды.
2. Нысан жойылған кезде барлық оған сілтеме жасап тұрған әлсіз көрсеткіштер автоматты түрде `nil` (нөл) болады.
3. Бұл үшін runtime жүйесінде орталықтандырылған **Weak Table** (әлсіз сілтемелер кестесі) қолданылады.

---

## 4. Көп ағындылық (Concurrency)

Butaq тілінде жаңа ағын `ағын` кілт сөзі арқылы іске қосылады:
```butaq
ағын менің_функция(аргумент)
```
Параллельді орындалуды басқару үшін жүйеде `ағын_күту` функциясы қолданылады.
- **Windows**: OS API `CreateThread` және `WaitForSingleObject` қолданады.
- **Linux**: POSIX `pthread_create` және `pthread_join` қолданады.
