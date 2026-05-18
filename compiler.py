from lexer import Token

# Opcodes
PUSH = "PUSH"
ADD = "ADD"
SUB = "SUB"
MUL = "MUL"
DIV = "DIV"
MOD = "MOD"
DUP = "DUP"
DROP = "DROP"
SWAP = "SWAP"
OVER = "OVER"
PRINT = "PRINT"
READ = "READ"
EQ = "EQ"
GT = "GT"
LT = "LT"
JMP = "JMP"
JMP_IF_FALSE = "JMP_IF_FALSE"
CALL = "CALL"
RETURN = "RETURN"

class Compiler:
    def __init__(self, tokens):
        self.tokens = tokens
        self.bytecode = []
        self.functions = {} # name -> start index
        self.loops = [] # stack for loop starts
        self.ifs = [] # stack for if jumps to patch

    def compile(self):
        i = 0
        while i < len(self.tokens):
            token = self.tokens[i]

            if token.type == "NUMBER" or token.type == "STRING":
                self.bytecode.append((PUSH, token.value))
            elif token.type == "WORD":
                word = token.value.lower()
                if word == "қосу":
                    self.bytecode.append((ADD,))
                elif word == "алу":
                    self.bytecode.append((SUB,))
                elif word == "көбейту":
                    self.bytecode.append((MUL,))
                elif word == "бөлу":
                    self.bytecode.append((DIV,))
                elif word == "қалдық":
                    self.bytecode.append((MOD,))
                elif word == "көшіру":
                    self.bytecode.append((DUP,))
                elif word == "тастау":
                    self.bytecode.append((DROP,))
                elif word == "ауыстыру":
                    self.bytecode.append((SWAP,))
                elif word == "астыңғы":
                    self.bytecode.append((OVER,))
                elif word == "жазу":
                    self.bytecode.append((PRINT,))
                elif word == "оқу":
                    self.bytecode.append((READ,))
                elif word == "тең":
                    self.bytecode.append((EQ,))
                elif word == "үлкен":
                    self.bytecode.append((GT,))
                elif word == "кіші":
                    self.bytecode.append((LT,))

                # Control flow
                elif word == "егер":
                    self.bytecode.append((JMP_IF_FALSE, -1))
                    self.ifs.append(('if', len(self.bytecode) - 1))

                elif word == "әйтпесе":
                    self.bytecode.append((JMP, -1))
                    jmp_idx = len(self.bytecode) - 1

                    block_type, if_idx = self.ifs.pop()
                    self.bytecode[if_idx] = (JMP_IF_FALSE, len(self.bytecode))

                    self.ifs.append(('else', jmp_idx))

                elif word == "басы":
                    self.loops.append(len(self.bytecode))

                elif word == "әзірше":
                    self.bytecode.append((JMP_IF_FALSE, -1))
                    self.ifs.append(('while', len(self.bytecode) - 1))

                elif word == "дегеніміз":
                    # Function definition
                    if i == 0 or self.tokens[i-1].type != "WORD":
                        raise SyntaxError("Function name expected before 'дегеніміз'")
                    func_name = self.tokens[i-1].value.lower()

                    self.bytecode.pop() # remove the CALL to the function name

                    self.bytecode.append((JMP, -1))
                    func_jmp_idx = len(self.bytecode) - 1

                    self.functions[func_name] = len(self.bytecode)
                    self.ifs.append(('func', func_jmp_idx))

                elif word == "бітті":
                    if self.ifs:
                        block_type, last_idx = self.ifs.pop()
                        if block_type == 'func':
                            self.bytecode.append((RETURN,))
                            self.bytecode[last_idx] = (JMP, len(self.bytecode))
                        elif block_type == 'while':
                            loop_start = self.loops.pop()
                            self.bytecode.append((JMP, loop_start))
                            self.bytecode[last_idx] = (JMP_IF_FALSE, len(self.bytecode))
                        elif block_type == 'if' or block_type == 'else':
                            op = self.bytecode[last_idx][0]
                            self.bytecode[last_idx] = (op, len(self.bytecode))

                else:
                    self.bytecode.append((CALL, word))
            i += 1

        return self.bytecode, self.functions
