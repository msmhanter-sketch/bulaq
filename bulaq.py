import sys
from lexer import Lexer
from compiler import Compiler
from vm import VM

def run_code(source_code):
    try:
        lexer = Lexer(source_code)
        tokens = lexer.tokenize()

        compiler = Compiler(tokens)
        bytecode, functions = compiler.compile()

        vm = VM(bytecode, functions)
        vm.run()
    except Exception as e:
        print(f"Қате (Error): {e}")

def repl():
    print("Bulaq REPL-ге қош келдіңіз! (Шығу үшін 'шығу' деп жазыңыз)")

    # Persistent state for REPL
    compiler = Compiler([])
    vm = VM(compiler.bytecode, compiler.functions)

    while True:
        try:
            line = input("bulaq> ")
            if line.strip() == "шығу":
                break
            if not line.strip():
                continue

            lexer = Lexer(line)
            tokens = lexer.tokenize()

            # Extend tokens and recompile from where we left off
            compiler.tokens = tokens
            compiler.compile() # This appends to compiler.bytecode

            # Update VM references
            vm.bytecode = compiler.bytecode
            vm.functions = compiler.functions

            # Run VM from its current IP to the new end
            vm.run()

        except Exception as e:
            print(f"Қате (Error): {e}")

def main():
    if len(sys.argv) > 1:
        filename = sys.argv[1]
        try:
            with open(filename, 'r', encoding='utf-8') as f:
                source_code = f.read()
            run_code(source_code)
        except FileNotFoundError:
            print(f"Файл табылмады (File not found): {filename}")
    else:
        repl()

if __name__ == "__main__":
    main()
