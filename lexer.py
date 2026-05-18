import re

class Token:
    def __init__(self, type, value, line, column):
        self.type = type
        self.value = value
        self.line = line
        self.column = column

    def __repr__(self):
        return f"Token({self.type}, {repr(self.value)})"

class Lexer:
    def __init__(self, source_code):
        self.source_code = source_code
        self.tokens = []

        # Regex patterns
        self.rules = [
            ("COMMENT", r'#.*'),
            ("STRING", r'"[^"]*"'),
            ("NUMBER", r'\d+(\.\d+)?'),
            ("WORD", r'[a-zA-Zа-яА-ЯәӘіІңҢғҒүҮұҰқҚөӨһҺ_]+'),
            ("WHITESPACE", r'\s+'),
            ("SYMBOL", r'[^\s\w]+'),
        ]

    def tokenize(self):
        pos = 0
        line = 1
        column = 1

        while pos < len(self.source_code):
            match = None
            for token_type, pattern in self.rules:
                regex = re.compile(pattern)
                match = regex.match(self.source_code, pos)

                if match:
                    raw_value = match.group(0)
                    value = raw_value
                    if token_type != "WHITESPACE" and token_type != "COMMENT":
                        if token_type == "STRING":
                            value = raw_value[1:-1] # Remove quotes
                        elif token_type == "NUMBER":
                            value = float(raw_value) if '.' in raw_value else int(raw_value)

                        self.tokens.append(Token(token_type, value, line, column))

                    pos = match.end(0)

                    # Update line and column
                    newlines = raw_value.count('\n')
                    if newlines > 0:
                        line += newlines
                        column = len(raw_value) - raw_value.rfind('\n')
                    else:
                        column += len(raw_value)

                    break

            if not match:
                raise SyntaxError(f"Lexer error at line {line}, column {column}: Unexpected character '{self.source_code[pos]}'")

        return self.tokens
