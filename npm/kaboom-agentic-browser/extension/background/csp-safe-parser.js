// csp-safe-parser.ts — Recursive descent parser for JS expressions into structured commands.
import { errorMessage } from '../lib/error-utils.js';
// Patterns that we reject outright before attempting to parse
const REJECTED_PATTERNS = [
    { pattern: /^\s*$/, reason: 'Empty expression' },
    { pattern: /^\s*(var|let|const)\s/, reason: 'Variable declarations are not supported' },
    { pattern: /^\s*if\s*\(/, reason: 'Control flow (if) is not supported' },
    { pattern: /^\s*for\s*\(/, reason: 'Control flow (for) is not supported' },
    { pattern: /^\s*while\s*\(/, reason: 'Control flow (while) is not supported' },
    { pattern: /^\s*switch\s*\(/, reason: 'Control flow (switch) is not supported' },
    { pattern: /^\s*async\s/, reason: 'async/await is not supported' },
    { pattern: /^\s*await\s/, reason: 'async/await is not supported' },
    { pattern: /^\s*function\s/, reason: 'Function declarations are not supported' },
    { pattern: /^\s*class\s/, reason: 'Class declarations are not supported' },
    { pattern: /=>/, reason: 'Arrow functions are not supported' },
    { pattern: /\.\.\.[a-zA-Z_$]/, reason: 'Spread/rest syntax is not supported' }
];
export function parseExpression(input) {
    // Strip 'return ' prefix
    let source = input.trim();
    if (source.endsWith(';'))
        source = source.slice(0, -1).trimEnd();
    if (source.startsWith('return '))
        source = source.slice(7).trimStart();
    // Check rejected patterns
    for (const { pattern, reason } of REJECTED_PATTERNS) {
        if (pattern.test(source)) {
            return { ok: false, reason };
        }
    }
    const parser = new Parser(source);
    try {
        const expr = parser.parseExpression();
        parser.skipWhitespace();
        // Check for assignment: expr = value
        if (parser.peek() === '=') {
            parser.advance(); // consume '='
            // Make sure it's not == or ===
            if (parser.peek() === '=') {
                return { ok: false, reason: 'Comparison operators are not supported' };
            }
            parser.skipWhitespace();
            const rhs = parser.parseExpression();
            parser.skipWhitespace();
            if (parser.pos < parser.source.length) {
                return { ok: false, reason: `Unexpected characters after expression: ${parser.source.slice(parser.pos)}` };
            }
            // Decompose the LHS into target + steps + key
            const assign = decomposeAssignment(expr);
            if (!assign) {
                return { ok: false, reason: 'Invalid assignment target' };
            }
            return { ok: true, command: { expr: rhs, assign } };
        }
        if (parser.pos < parser.source.length) {
            const remaining = parser.source.slice(parser.pos).trim();
            // Check for operators
            if (/^[+\-*/%&|^<>!?~]/.test(remaining)) {
                return { ok: false, reason: 'Operators are not supported' };
            }
            return { ok: false, reason: `Unexpected characters after expression: ${remaining}` };
        }
        return { ok: true, command: { expr } };
    }
    catch (e) {
        return { ok: false, reason: errorMessage(e) };
    }
}
function decomposeAssignment(expr) {
    if (expr.type === 'chain') {
        const steps = [...expr.steps];
        const lastStep = steps.pop();
        if (!lastStep || lastStep.op !== 'access')
            return null;
        return { target: expr.root, steps, key: lastStep.key };
    }
    return null;
}
class Parser {
    source;
    pos;
    constructor(source) {
        this.source = source;
        this.pos = 0;
    }
    peek() {
        return this.source[this.pos] || '';
    }
    advance() {
        return this.source[this.pos++] || '';
    }
    skipWhitespace() {
        while (this.pos < this.source.length && /\s/.test(this.source[this.pos])) {
            this.pos++;
        }
    }
    parseExpression() {
        this.skipWhitespace();
        const primary = this.parsePrimary();
        return this.parseSuffix(primary);
    }
    parsePrimary() {
        this.skipWhitespace();
        const ch = this.peek();
        // String literals
        if (ch === "'" || ch === '"')
            return this.parseString();
        // Backtick template literal (simple, no interpolation)
        if (ch === '`')
            return this.parseTemplateLiteral();
        // Array literal
        if (ch === '[')
            return this.parseArray();
        // Parenthesized expression or object literal in parens
        if (ch === '(')
            return this.parseParenthesized();
        // Negative number
        if (ch === '-' && this.pos + 1 < this.source.length && /\d/.test(this.source[this.pos + 1])) {
            return this.parseNumber();
        }
        // Number
        if (/\d/.test(ch))
            return this.parseNumber();
        // new keyword
        if (this.matchKeyword('new'))
            return this.parseNew();
        // Identifier (global reference, keyword literal, etc.)
        if (/[a-zA-Z_$]/.test(ch))
            return this.parseIdentifier();
        throw new Error(`Unexpected character: ${ch}`);
    }
    parseSuffix(base) {
        const steps = [];
        let current = base;
        // Unwrap if base already has a chain — we'll extend its steps
        if (current.type === 'chain') {
            steps.push(...current.steps);
            current = current.root;
        }
        while (this.pos < this.source.length) {
            this.skipWhitespace();
            const ch = this.peek();
            // .property
            if (ch === '.') {
                this.advance();
                const key = this.readIdentifier();
                if (!key)
                    throw new Error('Expected property name after "."');
                steps.push({ op: 'access', key });
                continue;
            }
            // [key] or [index]
            if (ch === '[') {
                this.advance();
                this.skipWhitespace();
                const innerCh = this.peek();
                if (innerCh === "'" || innerCh === '"') {
                    // String key
                    const strVal = this.parseString();
                    if (strVal.type !== 'literal' || typeof strVal.value !== 'string') {
                        throw new Error('Expected string key in bracket access');
                    }
                    this.skipWhitespace();
                    this.expect(']');
                    steps.push({ op: 'access', key: strVal.value });
                }
                else if (/\d/.test(innerCh) || innerCh === '-') {
                    // Numeric index
                    const numVal = this.parseNumber();
                    if (numVal.type !== 'literal' || typeof numVal.value !== 'number') {
                        throw new Error('Expected numeric index in bracket access');
                    }
                    this.skipWhitespace();
                    this.expect(']');
                    steps.push({ op: 'index', index: numVal.value });
                }
                else {
                    throw new Error('Bracket access only supports string keys and numeric indices');
                }
                continue;
            }
            // (args) — function call
            if (ch === '(') {
                const args = this.parseArgList();
                steps.push({ op: 'call', args });
                continue;
            }
            break;
        }
        if (steps.length === 0)
            return current;
        return { type: 'chain', root: current, steps };
    }
    parseString() {
        const quote = this.advance(); // consume opening quote
        let value = '';
        while (this.pos < this.source.length) {
            const ch = this.advance();
            if (ch === '\\') {
                const next = this.advance();
                if (next === 'n')
                    value += '\n';
                else if (next === 't')
                    value += '\t';
                else if (next === 'r')
                    value += '\r';
                else
                    value += next;
            }
            else if (ch === quote) {
                return { type: 'literal', value };
            }
            else {
                value += ch;
            }
        }
        throw new Error('Unterminated string literal');
    }
    parseTemplateLiteral() {
        this.advance(); // consume opening backtick
        let value = '';
        while (this.pos < this.source.length) {
            const ch = this.advance();
            if (ch === '`') {
                return { type: 'literal', value };
            }
            if (ch === '$' && this.peek() === '{') {
                throw new Error('Template literal interpolation is not supported');
            }
            if (ch === '\\') {
                const next = this.advance();
                if (next === 'n')
                    value += '\n';
                else if (next === 't')
                    value += '\t';
                else if (next === 'r')
                    value += '\r';
                else
                    value += next;
            }
            else {
                value += ch;
            }
        }
        throw new Error('Unterminated template literal');
    }
    parseNumber() {
        let numStr = '';
        if (this.peek() === '-')
            numStr += this.advance();
        while (this.pos < this.source.length && /[\d.]/.test(this.peek())) {
            numStr += this.advance();
        }
        const value = Number(numStr);
        if (Number.isNaN(value))
            throw new Error(`Invalid number: ${numStr}`);
        return { type: 'literal', value };
    }
    parseArray() {
        this.advance(); // consume '['
        this.skipWhitespace();
        if (this.peek() === ']') {
            this.advance();
            return { type: 'array', elements: [] };
        }
        // Check for spread
        if (this.peek() === '.' &&
            this.pos + 2 < this.source.length &&
            this.source[this.pos + 1] === '.' &&
            this.source[this.pos + 2] === '.') {
            throw new Error('Spread/rest syntax is not supported');
        }
        const elements = [];
        while (true) {
            this.skipWhitespace();
            if (this.peek() === '.' &&
                this.pos + 2 < this.source.length &&
                this.source[this.pos + 1] === '.' &&
                this.source[this.pos + 2] === '.') {
                throw new Error('Spread/rest syntax is not supported');
            }
            elements.push(this.parseExpression());
            this.skipWhitespace();
            if (this.peek() === ',') {
                this.advance();
                continue;
            }
            if (this.peek() === ']') {
                this.advance();
                break;
            }
            throw new Error('Expected "," or "]" in array literal');
        }
        return { type: 'array', elements };
    }
    parseParenthesized() {
        this.advance(); // consume '('
        this.skipWhitespace();
        // Check for IIFE: (function...)
        if (this.matchKeywordNoConsume('function')) {
            throw new Error('IIFE is not supported');
        }
        // Check for object literal: ({...})
        if (this.peek() === '{') {
            const obj = this.parseObject();
            this.skipWhitespace();
            this.expect(')');
            return obj;
        }
        // Regular parenthesized expression
        const expr = this.parseExpression();
        this.skipWhitespace();
        this.expect(')');
        return expr;
    }
    parseObject() {
        this.advance(); // consume '{'
        this.skipWhitespace();
        if (this.peek() === '}') {
            this.advance();
            return { type: 'object', entries: [] };
        }
        const entries = [];
        while (true) {
            this.skipWhitespace();
            const key = this.parseObjectKey();
            this.skipWhitespace();
            this.expect(':');
            this.skipWhitespace();
            const value = this.parseExpression();
            entries.push({ key, value });
            this.skipWhitespace();
            if (this.peek() === ',') {
                this.advance();
                this.skipWhitespace();
                if (this.peek() === '}') {
                    this.advance(); // trailing comma
                    break;
                }
                continue;
            }
            if (this.peek() === '}') {
                this.advance();
                break;
            }
            throw new Error('Expected "," or "}" in object literal');
        }
        return { type: 'object', entries };
    }
    parseObjectKey() {
        const ch = this.peek();
        if (ch === "'" || ch === '"') {
            const strVal = this.parseString();
            if (strVal.type !== 'literal' || typeof strVal.value !== 'string') {
                throw new Error('Expected string object key');
            }
            return strVal.value;
        }
        // Identifier key
        const key = this.readIdentifier();
        if (!key)
            throw new Error('Expected object key');
        return key;
    }
    parseNew() {
        // 'new' keyword already matched and consumed by matchKeyword
        this.skipWhitespace();
        const name = this.readIdentifier();
        if (!name)
            throw new Error('Expected constructor name after "new"');
        // May have dotted path: new some.Module()
        let root = { type: 'global', name };
        const steps = [];
        while (this.peek() === '.') {
            this.advance();
            const key = this.readIdentifier();
            if (!key)
                throw new Error('Expected property name after "."');
            steps.push({ op: 'access', key });
        }
        // Parse constructor args
        const args = this.parseArgList();
        steps.push({ op: 'construct', args });
        return { type: 'chain', root, steps };
    }
    parseIdentifier() {
        const name = this.readIdentifier();
        if (!name)
            throw new Error('Expected identifier');
        // Keyword literals
        if (name === 'true')
            return { type: 'literal', value: true };
        if (name === 'false')
            return { type: 'literal', value: false };
        if (name === 'null')
            return { type: 'literal', value: null };
        if (name === 'undefined')
            return { type: 'undefined' };
        return { type: 'global', name };
    }
    parseArgList() {
        this.expect('(');
        this.skipWhitespace();
        if (this.peek() === ')') {
            this.advance();
            return [];
        }
        const args = [];
        while (true) {
            this.skipWhitespace();
            args.push(this.parseExpression());
            this.skipWhitespace();
            if (this.peek() === ',') {
                this.advance();
                continue;
            }
            if (this.peek() === ')') {
                this.advance();
                break;
            }
            throw new Error('Expected "," or ")" in argument list');
        }
        return args;
    }
    readIdentifier() {
        let name = '';
        while (this.pos < this.source.length && /[a-zA-Z0-9_$]/.test(this.peek())) {
            name += this.advance();
        }
        return name;
    }
    matchKeyword(keyword) {
        const start = this.pos;
        this.skipWhitespace();
        if (this.source.startsWith(keyword, this.pos)) {
            const afterKeyword = this.pos + keyword.length;
            // Ensure it's not part of a longer identifier
            if (afterKeyword >= this.source.length || !/[a-zA-Z0-9_$]/.test(this.source[afterKeyword])) {
                this.pos = afterKeyword;
                return true;
            }
        }
        this.pos = start;
        return false;
    }
    matchKeywordNoConsume(keyword) {
        const saved = this.pos;
        const result = this.matchKeyword(keyword);
        this.pos = saved;
        return result;
    }
    expect(ch) {
        if (this.peek() !== ch) {
            throw new Error(`Expected "${ch}" but got "${this.peek() || 'EOF'}"`);
        }
        this.advance();
    }
}
//# sourceMappingURL=csp-safe-parser.js.map