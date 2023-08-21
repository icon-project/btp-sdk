// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.0;

import "@iconfoundation/btp2-solidity-library/contracts/interfaces/ICallService.sol";
import "@iconfoundation/btp2-solidity-library/contracts/interfaces/ICallServiceReceiver.sol";
import "@iconfoundation/btp2-solidity-library/contracts/utils/RLPEncode.sol";
import "@iconfoundation/btp2-solidity-library/contracts/utils/RLPDecode.sol";
import "@iconfoundation/btp2-solidity-library/contracts/utils/Strings.sol";
import "@iconfoundation/btp2-solidity-library/contracts/utils/BTPAddress.sol";
import "@iconfoundation/btp2-solidity-library/contracts/utils/ParseAddress.sol";

import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";

contract DAppSample is ICallServiceReceiver, Initializable {
    using RLPEncode for bytes;
    using RLPEncode for string;
    using RLPEncode for uint256;
    using RLPEncode for int256;
    using RLPEncode for address;
    using RLPEncode for bool;
    using RLPDecode for RLPDecode.RLPItem;
    using RLPDecode for RLPDecode.Iterator;
    using RLPDecode for bytes;
    using BTPAddress for string;
    using ParseAddress for address;

    address private xCall;
    string private xCallBTPAddress;
    uint256 lastSn;
    uint256 constant ROLLBACK_DISABLED = 0;
    uint256 constant ROLLBACK_ENABLED = 1;
    uint256 constant ROLLBACK_RESERVED = 2;
    string constant SHOULD_REVERT = "shouldRevert";
    mapping(uint256 => Message) rollbacks; //map sn to Message(xcallSn, _rollback)

    function initialize(
    ) public initializer {
    }

    function ensureXCall(
    ) private view returns (address) {
        require(xCall != address(0), "requireXCall");
        return xCall;
    }

    modifier onlyXCall(
    ) {
        require(msg.sender == ensureXCall(), "onlyXCall");
        _;
    }

    function setXCall(
        address _addr
    ) external {
        xCall = _addr;
        if (_addr != address(0)) {
            xCallBTPAddress = ICallService(_addr).getBtpAddress();
        } else {
            xCallBTPAddress = "";
        }
    }

    function getXCall(
    ) external view returns (address) {
        return xCall;
    }

    function getBTPAddress(
    ) external view returns (string memory) {
        if (bytes(xCallBTPAddress).length == 0) {
            return "";
        }
        return xCallBTPAddress.networkAddress().btpAddress(address(this).toString());
    }

    function getLastSn(
    ) external view returns (uint256) {
        return lastSn;
    }

    function nextSn(
    ) internal returns (uint256) {
        lastSn = lastSn + 1;
        return lastSn;
    }

    function sendMessage(
        string memory _to,
        bytes memory _data
    ) external payable {
        bytes memory _rollback;
        _sendMessage(_to, _data, _rollback);
    }

    function sendMessage(
        string memory _to,
        bytes memory _data,
        bytes memory _rollback
    ) external payable {
        _sendMessage(_to, _data, _rollback);
    }

    function _sendMessage(
        string memory _to,
        bytes memory _data,
        bytes memory _rollback
    ) private {
        address addr = ensureXCall();
        uint256 sn = nextSn();
        bytes memory data = encodeMessage(Message(sn, _data));
        bytes memory rollback;
        uint256 rollbackFlag = ROLLBACK_DISABLED;
        if (_rollback.length > 0) {
            rollback = abi.encode(sn, _rollback);
            rollbackFlag = ROLLBACK_ENABLED;
        }
        uint256 xcallSn = ICallService(addr).sendCallMessage{value:msg.value}(_to, data, rollback);

        // The code below is not actually necessary because the _rollback data is stored on the xCall side,
        // but in this example, it is needed for testing to compare the _rollback data later.
        if (Strings.compareTo(SHOULD_REVERT, string(_data))) {
            rollbacks[sn] = Message(xcallSn, _rollback);
            rollbackFlag = ROLLBACK_RESERVED;
        }

        emit Sent(_to, sn, rollbackFlag, xcallSn);
    }

    event Sent(string _to, uint256 _sn, uint256 _rollback, uint256 _xcallSn);

    function handleCallMessage(
        string calldata _from,
        bytes calldata _data
    ) external override onlyXCall {
        if (Strings.compareTo(xCallBTPAddress, _from)) {
            // handle rollback data here
            // In this example, just compare it with the stored one.
            (uint256 sn, bytes memory rollbackData) = abi.decode(_data, (uint256, bytes));
            Message memory rollback = rollbacks[sn];
            require(rollback.data.length > 0, "unexpected rollback");
            require(Strings.compareTo(string(rollbackData), string(rollback.data)), "rollbackData mismatch");
            delete rollbacks[sn]; //cleanup
            emit RollbackDataReceived(sn, rollbackData, rollback.sn);
        } else {
            Message memory _msg = decodeMessage(_data);
            if (Strings.compareTo(SHOULD_REVERT, string(_msg.data))) {
                revert("revertFromDApp");
            }
            emit MessageReceived(_from, _msg.sn, _msg.data);
        }
    }

    event MessageReceived(string _from, uint256 _sn, bytes _data);
    event RollbackDataReceived(uint256 _sn, bytes _data, uint256 _xcallSn);

    struct Message {
        uint256 sn;
        bytes data;
    }

    function encodeMessage(
        Message memory _msg
    ) internal pure returns (bytes memory) {
        bytes memory _rlp = abi.encodePacked(
            _msg.sn.encodeUint(),
            _msg.data.encodeBytes()
        );
        return _rlp.encodeList();
    }

    function decodeMessage(
        bytes memory _rlp
    ) internal pure returns (Message memory) {
        RLPDecode.RLPItem[] memory ls = _rlp.toRlpItem().toList();
        return Message(
            ls[0].toUint(),
            ls[1].toBytes()
        );
    }
}
