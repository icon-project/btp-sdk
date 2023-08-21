/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package foundation.icon.btp.example;

import foundation.icon.btp.lib.BTPAddress;
import foundation.icon.btp.xcall.CallServiceReceiver;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;
import score.Address;
import score.ByteArrayObjectWriter;
import score.Context;
import score.DictDB;
import score.ObjectReader;
import score.ObjectWriter;
import score.UserRevertedException;
import score.VarDB;
import score.annotation.EventLog;
import score.annotation.External;
import score.annotation.Optional;
import score.annotation.Payable;

import java.math.BigInteger;
import java.util.Arrays;

public class DAppSample implements CallServiceReceiver {
    private final VarDB<Address> xCallVarDB = Context.newVarDB("xCall", Address.class);
    private final VarDB<String> xCallBTPAddressVarDB = Context.newVarDB("xCallBTPAddress", String.class);
    private final VarDB<BigInteger> lastSnVarDB = Context.newVarDB("sn", BigInteger.class);
    public static final BigInteger ROLLBACK_DISABLED = BigInteger.ZERO;
    public static final BigInteger ROLLBACK_ENABLED = BigInteger.ONE;
    public static final BigInteger ROLLBACK_RESERVED = BigInteger.TWO;
    public static final String SHOULD_REVERT = "shouldRevert";
    private final DictDB<BigInteger, Message> rollbacks = Context.newDictDB("rollbacks", Message.class);

    public DAppSample() {
    }

    private Address ensureXCall() {
        Address addr = xCallVarDB.get();
        Context.require(addr != null, "requireXCall");
        return addr;
    }

    private void onlyXCall() {
        Context.require(Context.getCaller().equals(ensureXCall()), "onlyXCall");
    }

    @External
    public void setXCall(Address _addr) {
        xCallVarDB.set(_addr);
        if (_addr != null) {
            xCallBTPAddressVarDB.set(new XCallScoreInterface(_addr).getBtpAddress());
        } else {
            xCallBTPAddressVarDB.set(null);
        }
    }

    @External(readonly = true)
    public Address getXCall() {
        return xCallVarDB.get();
    }

    @External(readonly = true)
    public String getBTPAddress() {
        String xCallBTPAddress = xCallBTPAddressVarDB.get();
        if (xCallBTPAddress == null) {
            return null;
        }
        return new BTPAddress(BTPAddress.parse(xCallBTPAddressVarDB.get()).net(),
                Context.getAddress().toString()).toString();
    }

    @External(readonly = true)
    public BigInteger getLastSn() {
        return lastSnVarDB.getOrDefault(BigInteger.ZERO);
    }

    private BigInteger nextSn() {
        BigInteger sn = getLastSn().add(BigInteger.ONE);
        this.lastSnVarDB.set(sn);
        return sn;
    }

    @Payable
    @External
    public void sendMessage(String _to, byte[] _data, @Optional byte[] _rollback) {
        XCallScoreInterface xCall = new XCallScoreInterface(ensureXCall());
        try {
            BigInteger sn = nextSn();
            byte[] data = new Message(sn, _data).toBytes();
            byte[] rollback = null;
            BigInteger rollbackFlag = ROLLBACK_DISABLED;
            if (_rollback != null && _rollback.length > 0) {
                Message rollbackMsg = new Message(sn, _rollback);
                rollback = rollbackMsg.toBytes();
                rollbackFlag = ROLLBACK_ENABLED;
            }
            BigInteger xcallSn = xCall.sendCallMessage(Context.getValue(), _to, data, rollback);

            // The code below is not actually necessary because the _rollback data is stored on the xCall side,
            // but in this example, it is needed for testing to compare the _rollback data later.
            if (SHOULD_REVERT.equals(new String(_data))) {
                rollbacks.set(sn, new Message(xcallSn, _rollback));
                rollbackFlag = ROLLBACK_RESERVED;
            }

            Sent(_to, sn, rollbackFlag, xcallSn);
        } catch (UserRevertedException e) {
            Context.revert(e.getCode(), e.getMessage());
        }
    }

    @EventLog
    public void Sent(String _to, BigInteger _sn, BigInteger _rollback, BigInteger _xcallSn) {
    }

    @Override
    @External
    public void handleCallMessage(String _from, byte[] _data) {
        onlyXCall();
        Message msg = Message.fromBytes(_data);
        if (xCallBTPAddressVarDB.get().equals(_from)) {
            // handle rollback data here
            // In this example, just compare it with the stored one.
            Message rollback = rollbacks.get(msg.sn);
            Context.require(rollback != null, "unexpected rollback");
            Context.require(Arrays.equals(rollback.data, msg.data), "rollbackData mismatch");
            rollbacks.set(msg.sn, null); //cleanup
            RollbackDataReceived(msg.sn, msg.data, rollback.sn);
        } else {
            if (SHOULD_REVERT.equals(new String(msg.data))) {
                Context.revert("revertFromDApp");
            }
            MessageReceived(_from, msg.sn, msg.data);
        }
    }

    @EventLog
    public void MessageReceived(String _from, BigInteger _sn, byte[] _data) {
    }

    @EventLog
    public void RollbackDataReceived(BigInteger _sn, byte[] _data, BigInteger _xcallSn) {
    }

    @AllArgsConstructor
    @NoArgsConstructor
    public static class Message {
        BigInteger sn;
        byte[] data;

        public static void writeObject(ObjectWriter writer, Message obj) {
            obj.writeObject(writer);
        }

        public static Message readObject(ObjectReader reader) {
            Message obj = new Message();
            reader.beginList();
            obj.sn = reader.readNullable(BigInteger.class);
            obj.data = reader.readNullable(byte[].class);
            reader.end();
            return obj;
        }

        public void writeObject(ObjectWriter writer) {
            writer.beginList(2);
            writer.writeNullable(this.sn);
            writer.writeNullable(this.data);
            writer.end();
        }

        public static Message fromBytes(byte[] bytes) {
            ObjectReader reader = Context.newByteArrayObjectReader("RLPn", bytes);
            return Message.readObject(reader);
        }

        public byte[] toBytes() {
            ByteArrayObjectWriter writer = Context.newByteArrayObjectWriter("RLPn");
            Message.writeObject(writer, this);
            return writer.toByteArray();
        }
    }
}
