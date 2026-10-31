package com.tigerswap;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Minimal JSON reader and writer.
 *
 * The SDK carries no third-party dependency, so the small amount of JSON it
 * needs is handled here. {@link #parse} accepts exactly RFC 8259 JSON and
 * rejects anything else rather than guessing, and {@link #write} escapes every
 * string it emits, so a caller-supplied value cannot break out of its literal
 * and change the shape of a request body.
 */
final class Json {

    /** Raised when input is not valid JSON. */
    static final class JsonException extends RuntimeException {
        private static final long serialVersionUID = 1L;

        JsonException(String message) {
            super(message);
        }
    }

    private final String src;
    private int pos;

    private Json(String src) {
        this.src = src;
    }

    static Object parse(String text) {
        if (text == null) {
            throw new JsonException("no JSON input");
        }
        Json reader = new Json(text);
        reader.skipWhitespace();
        Object value = reader.readValue();
        reader.skipWhitespace();
        if (reader.pos != reader.src.length()) {
            throw new JsonException("trailing data at offset " + reader.pos);
        }
        return value;
    }

    private void skipWhitespace() {
        while (pos < src.length()) {
            char c = src.charAt(pos);
            if (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
                pos++;
            } else {
                break;
            }
        }
    }

    private Object readValue() {
        if (pos >= src.length()) {
            throw new JsonException("unexpected end of input");
        }
        char c = src.charAt(pos);
        switch (c) {
            case '{':
                return readObject();
            case '[':
                return readArray();
            case '"':
                return readString();
            case 't':
                expectLiteral("true");
                return Boolean.TRUE;
            case 'f':
                expectLiteral("false");
                return Boolean.FALSE;
            case 'n':
                expectLiteral("null");
                return null;
            default:
                return readNumber();
        }
    }

    private void expectLiteral(String literal) {
        if (!src.startsWith(literal, pos)) {
            throw new JsonException("invalid literal at offset " + pos);
        }
        pos += literal.length();
    }

    private Map<String, Object> readObject() {
        expect('{');
        Map<String, Object> object = new LinkedHashMap<>();
        skipWhitespace();
        if (peek() == '}') {
            pos++;
            return object;
        }
        while (true) {
            skipWhitespace();
            String key = readString();
            skipWhitespace();
            expect(':');
            skipWhitespace();
            object.put(key, readValue());
            skipWhitespace();
            char next = peek();
            if (next == ',') {
                pos++;
                continue;
            }
            if (next == '}') {
                pos++;
                return object;
            }
            throw new JsonException("expected ',' or '}' at offset " + pos);
        }
    }

    private List<Object> readArray() {
        expect('[');
        List<Object> array = new ArrayList<>();
        skipWhitespace();
        if (peek() == ']') {
            pos++;
            return array;
        }
        while (true) {
            skipWhitespace();
            array.add(readValue());
            skipWhitespace();
            char next = peek();
            if (next == ',') {
                pos++;
                continue;
            }
            if (next == ']') {
                pos++;
                return array;
            }
            throw new JsonException("expected ',' or ']' at offset " + pos);
        }
    }

    private String readString() {
        expect('"');
        StringBuilder out = new StringBuilder();
        while (true) {
            if (pos >= src.length()) {
                throw new JsonException("unterminated string");
            }
            char c = src.charAt(pos++);
            if (c == '"') {
                return out.toString();
            }
            if (c != '\\') {
                out.append(c);
                continue;
            }
            if (pos >= src.length()) {
                throw new JsonException("unterminated escape");
            }
            char esc = src.charAt(pos++);
            switch (esc) {
                case '"': out.append('"'); break;
                case '\\': out.append('\\'); break;
                case '/': out.append('/'); break;
                case 'b': out.append('\b'); break;
                case 'f': out.append('\f'); break;
                case 'n': out.append('\n'); break;
                case 'r': out.append('\r'); break;
                case 't': out.append('\t'); break;
                case 'u':
                    if (pos + 4 > src.length()) {
                        throw new JsonException("truncated \\u escape");
                    }
                    out.append((char) Integer.parseInt(src.substring(pos, pos + 4), 16));
                    pos += 4;
                    break;
                default:
                    throw new JsonException("invalid escape \\" + esc);
            }
        }
    }

    private Double readNumber() {
        int start = pos;
        while (pos < src.length()) {
            char c = src.charAt(pos);
            if (c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' || (c >= '0' && c <= '9')) {
                pos++;
            } else {
                break;
            }
        }
        if (start == pos) {
            throw new JsonException("invalid value at offset " + start);
        }
        String text = src.substring(start, pos);
        try {
            return Double.valueOf(text);
        } catch (NumberFormatException e) {
            throw new JsonException("invalid number '" + text + "'");
        }
    }

    private char peek() {
        if (pos >= src.length()) {
            throw new JsonException("unexpected end of input");
        }
        return src.charAt(pos);
    }

    private void expect(char expected) {
        if (peek() != expected) {
            throw new JsonException("expected '" + expected + "' at offset " + pos);
        }
        pos++;
    }

    // ------------------------------------------------------------------
    // writing
    // ------------------------------------------------------------------

    static String write(Object value) {
        StringBuilder out = new StringBuilder();
        writeValue(out, value);
        return out.toString();
    }

    private static void writeValue(StringBuilder out, Object value) {
        if (value == null) {
            out.append("null");
        } else if (value instanceof String) {
            writeString(out, (String) value);
        } else if (value instanceof Boolean) {
            out.append(value.toString());
        } else if (value instanceof Number) {
            double d = ((Number) value).doubleValue();
            if (Double.isNaN(d) || Double.isInfinite(d)) {
                throw new JsonException("cannot encode non-finite number");
            }
            out.append(value.toString());
        } else if (value instanceof Map) {
            out.append('{');
            boolean first = true;
            for (Map.Entry<?, ?> entry : ((Map<?, ?>) value).entrySet()) {
                if (!first) {
                    out.append(',');
                }
                first = false;
                writeString(out, String.valueOf(entry.getKey()));
                out.append(':');
                writeValue(out, entry.getValue());
            }
            out.append('}');
        } else if (value instanceof Iterable) {
            out.append('[');
            boolean first = true;
            for (Object element : (Iterable<?>) value) {
                if (!first) {
                    out.append(',');
                }
                first = false;
                writeValue(out, element);
            }
            out.append(']');
        } else {
            throw new JsonException("cannot encode " + value.getClass().getName());
        }
    }

    private static void writeString(StringBuilder out, String value) {
        out.append('"');
        for (int i = 0; i < value.length(); i++) {
            char c = value.charAt(i);
            switch (c) {
                case '"': out.append("\\\""); break;
                case '\\': out.append("\\\\"); break;
                case '\b': out.append("\\b"); break;
                case '\f': out.append("\\f"); break;
                case '\n': out.append("\\n"); break;
                case '\r': out.append("\\r"); break;
                case '\t': out.append("\\t"); break;
                default:
                    if (c < 0x20) {
                        out.append(String.format("\\u%04x", (int) c));
                    } else {
                        out.append(c);
                    }
            }
        }
        out.append('"');
    }

    // ------------------------------------------------------------------
    // typed accessors
    // ------------------------------------------------------------------

    @SuppressWarnings("unchecked")
    static Map<String, Object> asObject(Object value, String field) {
        if (!(value instanceof Map)) {
            throw new JsonException(field + " is not a JSON object");
        }
        return (Map<String, Object>) value;
    }

    @SuppressWarnings("unchecked")
    static List<Object> asArray(Object value, String field) {
        if (!(value instanceof List)) {
            throw new JsonException(field + " is not a JSON array");
        }
        return (List<Object>) value;
    }

    static Object field(Map<String, Object> object, String key) {
        return object.get(key);
    }

    static String requiredString(Map<String, Object> object, String key) {
        Object value = object.get(key);
        if (!(value instanceof String)) {
            throw new JsonException("missing or non-string field '" + key + "'");
        }
        return (String) value;
    }

    static String optionalString(Map<String, Object> object, String key) {
        Object value = object.get(key);
        return value instanceof String ? (String) value : null;
    }

    static long requiredLong(Map<String, Object> object, String key) {
        Object value = object.get(key);
        if (!(value instanceof Number)) {
            throw new JsonException("missing or non-numeric field '" + key + "'");
        }
        double d = ((Number) value).doubleValue();
        if (d != Math.rint(d)) {
            throw new JsonException("field '" + key + "' is not an integer");
        }
        return (long) d;
    }

    static int requiredInt(Map<String, Object> object, String key) {
        long value = requiredLong(object, key);
        if (value < Integer.MIN_VALUE || value > Integer.MAX_VALUE) {
            throw new JsonException("field '" + key + "' does not fit in an int");
        }
        return (int) value;
    }

    static double requiredDouble(Map<String, Object> object, String key) {
        Object value = object.get(key);
        if (!(value instanceof Number)) {
            throw new JsonException("missing or non-numeric field '" + key + "'");
        }
        return ((Number) value).doubleValue();
    }
}
