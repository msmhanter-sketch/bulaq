from compiler import *

class VM:
    def __init__(self, bytecode, functions):
        self.bytecode = bytecode
        self.functions = functions
        self.stack = []
        self.call_stack = [] # to store return addresses
        self.ip = 0 # instruction pointer

    def run(self):
        while self.ip < len(self.bytecode):
            inst = self.bytecode[self.ip]
            op = inst[0]

            if op == PUSH:
                self.stack.append(inst[1])
            elif op == ADD:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(a + b)
            elif op == SUB:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(a - b)
            elif op == MUL:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(a * b)
            elif op == DIV:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(a / b)
            elif op == MOD:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(a % b)
            elif op == DUP:
                self.stack.append(self.stack[-1])
            elif op == DROP:
                self.stack.pop()
            elif op == SWAP:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(b)
                self.stack.append(a)
            elif op == OVER:
                self.stack.append(self.stack[-2])
            elif op == PRINT:
                print(self.stack.pop())
            elif op == READ:
                val = input()
                # Try to convert to float/int if possible
                try:
                    val = float(val) if '.' in val else int(val)
                except ValueError:
                    pass
                self.stack.append(val)
            elif op == EQ:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(1 if a == b else 0)
            elif op == GT:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(1 if a > b else 0)
            elif op == LT:
                b = self.stack.pop()
                a = self.stack.pop()
                self.stack.append(1 if a < b else 0)
            elif op == JMP:
                self.ip = inst[1]
                continue
            elif op == JMP_IF_FALSE:
                cond = self.stack.pop()
                if not cond:
                    self.ip = inst[1]
                    continue
            elif op == CALL:
                func_name = inst[1]
                if func_name in self.functions:
                    self.call_stack.append(self.ip + 1)
                    self.ip = self.functions[func_name]
                    continue
                else:
                    raise NameError(f"Belgisiz funksiya (Unknown function): {func_name}")
            elif op == RETURN:
                self.ip = self.call_stack.pop()
                continue

            self.ip += 1
