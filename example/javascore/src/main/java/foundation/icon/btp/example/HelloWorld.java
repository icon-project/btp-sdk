package foundation.icon.btp.example;

import lombok.AllArgsConstructor;
import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import score.Address;
import score.Context;
import score.VarDB;
import score.annotation.EventLog;
import score.annotation.External;
import score.annotation.Optional;

import java.math.BigInteger;

import static foundation.icon.btp.example.Encode.encode;

public class HelloWorld {
    private final VarDB<String> varDB = Context.newVarDB("value", String.class);

    public HelloWorld(String value) {
        if (varDB.get() == null) {
            varDB.set(value);
        } else {
            Context.println("ignore constructor argument");
        }
    }

    @External(readonly = true)
    public String name() {
        return varDB.get();
    }

    @External(readonly = true)
    public Integers callInteger(byte arg1, short arg2, int arg3, long arg4,
                                BigInteger arg5, BigInteger arg6, BigInteger arg7, BigInteger arg8) {
        Context.require(arg5.bitLength() < 24);
        Context.require(arg6.bitLength() < 40);
        Context.require(arg7.bitLength() < 72);
        Context.require(arg8.bitLength() < 256);
        return new Integers(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
    }

    @External(readonly = true)
    public UnsignedIntegers callUnsignedInteger(BigInteger arg1, char arg2, BigInteger arg3, BigInteger arg4,
                                                BigInteger arg5, BigInteger arg6, BigInteger arg7, BigInteger arg8) {
        return new UnsignedIntegers(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
    }

    @External(readonly = true)
    public Primitives callPrimitive(BigInteger arg1, boolean arg2, String arg3, byte[] arg4, Address arg5) {
        Context.require(arg1.bitLength() < 256);
        return new Primitives(arg1, arg2, arg3, arg4, arg5);
    }

    @External(readonly = true)
    public OutputStruct callStruct(InputStruct arg1) {
        OutputStruct out = new OutputStruct();
        out.booleanVal = arg1.booleanVal;
        return out;
    }

    @External(readonly = true)
    public PrimitiveArrays callPrimitiveArray(BigInteger[] arg1, boolean[] arg2, String[] arg3, byte[][] arg4, Address[] arg5) {
        for (BigInteger v : arg1) {
            Context.require(v.bitLength() < 256);
        }
        return new PrimitiveArrays(arg1, arg2, arg3, arg4, arg5);
    }

    @External(readonly = true)
    public String callOptional(@Optional String arg1) {
        return "callOptional(" + (arg1 == null ? "" : arg1) + ")";
    }

    @External
    public void invokeInteger(byte arg1, short arg2, int arg3, long arg4,
                              BigInteger arg5, BigInteger arg6, BigInteger arg7, BigInteger arg8) {
        Context.require(arg5.bitLength() < 24);
        Context.require(arg6.bitLength() < 40);
        Context.require(arg7.bitLength() < 72);
        Context.require(arg8.bitLength() < 256);
        IntegerEvent(arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8);
    }

    @EventLog(indexed = 3)
    public void IntegerEvent(byte arg1, short arg2, int arg3, long arg4,
                             BigInteger arg5, BigInteger arg6, BigInteger arg7, BigInteger arg8) {
    }

    @External
    public void invokePrimitive(BigInteger arg1, boolean arg2, String arg3, byte[] arg4, Address arg5) {
        Context.require(arg1.bitLength() < 256);
        PrimitiveEvent(arg1, arg2, arg3, arg4, arg5);
    }

    @EventLog(indexed = 3)
    public void PrimitiveEvent(BigInteger arg1, boolean arg2, String arg3, byte[] arg4, Address arg5) {

    }

    @External
    public OutputStruct invokeStruct(InputStruct arg1) {
        OutputStruct out = callStruct(arg1);
        StructEvent(encode(out));
        return out;
    }

    /**
     * Eventlog does not allow struct
     */
    @EventLog
    public void StructEvent(byte[] arg1) {

    }

    @External
    public void invokePrimitiveArray(BigInteger[] arg1, boolean[] arg2, String[] arg3, byte[][] arg4, Address[] arg5) {
        PrimitiveArrayEvent(encode(arg1), encode(arg2), encode(arg3), encode(arg4), encode(arg5));
    }

    /**
     * Eventlog does not allow array except byte array
     */
    @EventLog(indexed = 3)
    public void PrimitiveArrayEvent(byte[] arg1, byte[] arg2, byte[] arg3, byte[] arg4, byte[] arg5) {

    }

    @External
    public void invokeStructArray(InputStruct[] arg1, InputStruct[] arg2) {

    }

    /**
     * Eventlog does not allow array except byte array
     */
    @EventLog(indexed = 1)
    public void StructArrayEvent(byte[] arg1, byte[] arg2, byte[] arg3, byte[] arg4, byte[] arg5) {

    }

    @NoArgsConstructor
    @AllArgsConstructor
    @Getter
    @Setter
    @EqualsAndHashCode
    public static class Integers {
        private byte arg1;
        private short arg2;
        private int arg3;
        private long arg4;
        private BigInteger arg5;//int24
        private BigInteger arg6;//int40
        private BigInteger arg7;//int72
        private BigInteger arg8;//int256
    }

    @NoArgsConstructor
    @AllArgsConstructor
    @Getter
    @Setter
    @EqualsAndHashCode
    public static class UnsignedIntegers {
        private BigInteger arg1;//uint8
        private char arg2;//uint16
        private BigInteger arg3;//uint32
        private BigInteger arg4;//uint64
        private BigInteger arg5;//uint24
        private BigInteger arg6;//uint40
        private BigInteger arg7;//uint72
        private BigInteger arg8;//uint256

    }

    @NoArgsConstructor
    @AllArgsConstructor
    @Getter
    @Setter
    @EqualsAndHashCode
    public static class Primitives {
        private BigInteger arg1;
        private boolean arg2;
        private String arg3;
        private byte[] arg4;
        private Address arg5;
    }

    @Setter
    public static class InputStruct {
        protected Boolean booleanVal;
    }

    @Getter
    public static class OutputStruct {
        protected Boolean booleanVal;
    }

    @NoArgsConstructor
    @AllArgsConstructor
    @Getter
    @Setter
    public static class PrimitiveArrays {
        private BigInteger[] arg1;
        private boolean[] arg2;
        private String[] arg3;
        private byte[][] arg4;
        private Address[] arg5;
    }

}
