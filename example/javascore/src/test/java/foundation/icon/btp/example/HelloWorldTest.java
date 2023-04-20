package foundation.icon.btp.example;

import com.fasterxml.jackson.annotation.JsonAutoDetect;
import foundation.icon.jsonrpc.model.TransactionResult;
import foundation.icon.score.client.DefaultScoreClient;
import foundation.icon.score.client.ScoreClient;
import org.apache.commons.lang3.builder.ReflectionToStringBuilder;
import org.apache.commons.lang3.builder.ToStringStyle;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import score.Address;

import java.math.BigInteger;
import java.util.Objects;
import java.util.function.Consumer;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;

class HelloWorldTest {
    @ScoreClient
    static HelloWorld helloWorld;
    static HelloWorldScoreClient client;
    static char charVal = Character.MAX_VALUE;
    static byte byteVal = Byte.MAX_VALUE;
    static short shortVal = Short.MAX_VALUE;
    static int intVal = Integer.MAX_VALUE;
    static long longVal = Long.MAX_VALUE;
    static BigInteger int24Val = new BigInteger("7f" + "ff".repeat(2), 16);
    static BigInteger int40Val = new BigInteger("7f" + "ff".repeat(4), 16);
    static BigInteger int72Val = new BigInteger("7f" + "ff".repeat(8), 16);
    static BigInteger bigIntegerVal = new BigInteger("7f" + "ff".repeat(31), 16);
    static boolean booleanVal = true;
    static String stringVal = "string";
    static byte[] bytesVal = "bytes".getBytes();
    static Address addressVal = DefaultScoreClient.wallet(System.getProperties()).getAddress();
    static InputStruct inputStructVal = new InputStruct(booleanVal);
    static OutputStruct outputStructVal = new OutputStruct(inputStructVal);

    @BeforeAll
    static void beforeAll() {
        client = HelloWorldScoreClient._of("", System.getProperties(), "v");
    }

    static void print(Object obj) {
        System.out.println(obj == null ? "null" :
                ReflectionToStringBuilder.reflectionToString(obj, ToStringStyle.MULTI_LINE_STYLE));
    }

    @Test
    void callInteger() {
        HelloWorld.Integers p = new HelloWorld.Integers(byteVal, shortVal, intVal, longVal,
                int24Val, int40Val, int72Val, bigIntegerVal);
        HelloWorld.Integers ret = client.callInteger(p.getArg1(), p.getArg2(), p.getArg3(), p.getArg4(),
                p.getArg5(), p.getArg6(), p.getArg7(), p.getArg8());
        assertEquals(p, ret);
        print(ret);
    }

    @Test
    void callUnsignedInteger() {
        HelloWorld.UnsignedIntegers p = new HelloWorld.UnsignedIntegers(
                new BigInteger("ff", 16),
                charVal,
                new BigInteger("ff".repeat(4), 16),
                new BigInteger("ff".repeat(8), 16),
                new BigInteger("ff".repeat(3), 16),
                new BigInteger("ff".repeat(5), 16),
                new BigInteger("ff".repeat(9), 16),
                new BigInteger("ff".repeat(32), 16));
        HelloWorld.UnsignedIntegers ret = client.callUnsignedInteger(p.getArg1(), p.getArg2(), p.getArg3(), p.getArg4(),
                p.getArg5(), p.getArg6(), p.getArg7(), p.getArg8());
        assertEquals(p, ret);
        print(ret);
    }

    @Test
    void callPrimitive() {
        HelloWorld.Primitives p = new HelloWorld.Primitives(
                bigIntegerVal, booleanVal, stringVal, bytesVal, addressVal);
        HelloWorld.Primitives ret = client.callPrimitive(
                p.getArg1(), p.isArg2(), p.getArg3(), p.getArg4(), p.getArg5());
        assertEquals(p, ret);
        print(ret);
    }

    /**
     * Since HelloWorld.InputStruct has not Getter,
     * it makes json serialization failure.
     */
    @JsonAutoDetect(fieldVisibility = JsonAutoDetect.Visibility.ANY)
    public static class InputStruct extends HelloWorld.InputStruct {
        public InputStruct() {
        }

        public InputStruct(Boolean booleanVal) {
            super();
            setBooleanVal(booleanVal);
        }
    }

    public static class OutputStruct extends HelloWorld.OutputStruct {
        public OutputStruct() {
        }

        public OutputStruct(InputStruct inputStruct) {
            super();
            super.booleanVal = inputStruct.booleanVal;
        }

        @Override
        public boolean equals(Object obj) {
            if (obj instanceof HelloWorld.OutputStruct) {
                return Objects.equals(booleanVal, ((HelloWorld.OutputStruct) obj).getBooleanVal());
            }
            return super.equals(obj);
        }
    }

    @Test
    void callStruct() {
        HelloWorld.OutputStruct ret = client.callStruct(inputStructVal);
        assertEquals(outputStructVal, ret);
        print(ret);
    }

    @Test
    void callArray() {
        HelloWorld.Arrays p = new HelloWorld.Arrays(
                new BigInteger[]{bigIntegerVal}, new boolean[]{booleanVal}, new String[]{stringVal},
                new byte[][]{bytesVal}, new Address[]{addressVal}, new HelloWorld.OutputStruct[]{outputStructVal});
        HelloWorld.Arrays ret = client.callArray(
                p.getArg1(), p.getArg2(), p.getArg3(), p.getArg4(), p.getArg5(),
                new HelloWorld.InputStruct[]{inputStructVal});
        assertArrayEquals(p.getArg1(), ret.getArg1());
        assertArrayEquals(p.getArg2(), ret.getArg2());
        assertArrayEquals(p.getArg3(), ret.getArg3());
        assertArrayEquals(p.getArg4(), ret.getArg4());
        assertArrayEquals(p.getArg5(), ret.getArg5());
        assertArrayEquals(p.getArg6(), ret.getArg6());
        print(ret);
    }

    @Test
    void callOptional() {
        print(client.callOptional(null));
        print(client.callOptional(stringVal));
    }

    @Test
    void invokeInteger() {
        HelloWorld.Integers p = new HelloWorld.Integers(byteVal, shortVal, intVal, longVal,
                int24Val, int40Val, int72Val, bigIntegerVal);
        Consumer<TransactionResult> consumer = client.IntegerEvent(l -> {
            assertEquals(1, l.size());
            HelloWorldScoreClient.IntegerEvent el = l.get(0);
            HelloWorld.Integers actual = new HelloWorld.Integers(
                    el.getArg1(), el.getArg2(), el.getArg3(), el.getArg4(),
                    el.getArg5(), el.getArg6(), el.getArg7(), el.getArg8());
            assertEquals(p, actual);
            print(el);

        }, null);
        client.invokeInteger(consumer,
                p.getArg1(), p.getArg2(), p.getArg3(), p.getArg4(),
                p.getArg5(), p.getArg6(), p.getArg7(), p.getArg8());
    }

    @Test
    void invokePrimitive() {
        HelloWorld.Primitives p = new HelloWorld.Primitives(
                bigIntegerVal, booleanVal, stringVal, bytesVal, addressVal);
        Consumer<TransactionResult> consumer = client.PrimitiveEvent(l -> {
            assertEquals(1, l.size());
            HelloWorldScoreClient.PrimitiveEvent el = l.get(0);
            HelloWorld.Primitives actual = new HelloWorld.Primitives(
                    el.getArg1(), el.getArg2(), el.getArg3(), el.getArg4(), el.getArg5());
            assertEquals(p, actual);
            print(el);

        }, null);
        client.invokePrimitive(consumer,
                p.getArg1(), p.isArg2(), p.getArg3(), p.getArg4(), p.getArg5());
    }

    @Test
    void invokeStruct() {
        Consumer<TransactionResult> consumer = client.StructEvent(l -> {
            assertEquals(1, l.size());
            HelloWorldScoreClient.StructEvent el = l.get(0);
            assertArrayEquals(Encode.encode(outputStructVal), el.getArg1());
            print(el);
        }, null);
        client.invokeStruct(consumer,
                inputStructVal);
    }

    @Disabled("not implemented")
    @Test
    void invokeArray() {
        HelloWorld.Arrays p = new HelloWorld.Arrays(
                new BigInteger[]{bigIntegerVal}, new boolean[]{booleanVal}, new String[]{stringVal},
                new byte[][]{bytesVal}, new Address[]{addressVal}, new HelloWorld.OutputStruct[]{outputStructVal});
        Consumer<TransactionResult> consumer = client.ArrayEvent(l -> {
            assertEquals(1, l.size());
            HelloWorldScoreClient.ArrayEvent el = l.get(0);
            assertArrayEquals(Encode.encode(outputStructVal), el.getArg1());
            assertArrayEquals(Encode.encode(p.getArg1()), el.getArg1());
            assertArrayEquals(Encode.encode(p.getArg2()), el.getArg2());
            assertArrayEquals(Encode.encode(p.getArg3()), el.getArg3());
            assertArrayEquals(Encode.encode(p.getArg4()), el.getArg4());
            assertArrayEquals(Encode.encode(p.getArg5()), el.getArg5());
            assertArrayEquals(Encode.encode(p.getArg6()), el.getArg6());
            print(el);
        }, null);
        client.invokeArray(consumer,
                p.getArg1(), p.getArg2(), p.getArg3(), p.getArg4(), p.getArg5(),
                new HelloWorld.InputStruct[]{inputStructVal});

    }

    @Disabled("not implemented")
    @Test
    void invokeOptional() {
    }
}
