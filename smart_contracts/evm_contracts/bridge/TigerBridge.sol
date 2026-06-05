// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title TigerBridge
 * @notice Cross-chain bridge contract for asset transfers
 */
contract TigerBridge {
    enum Status { Pending, Confirmed, Executed, Failed }
    
    struct Transfer {
        address sender;
        address receiver;
        address token;
        uint256 amount;
        uint256 destChain;
        uint256 nonce;
        Status status;
        bytes32 hash;
        uint256 confirmations;
        mapping(address => bool) hasConfirmed;
    }

    uint256 public chainId;
    uint256 public requiredConfirmations = 3;
    uint256 public minTransferAmount = 1;
    uint256 public maxTransferAmount = 1000000 * 10**18;
    uint256 public feePercentage = 10; // 0.1%
    uint256 public constant GAS_RESERVE = 0.01 ether;
    
    mapping(bytes32 => Transfer) public transfers;
    mapping(uint256 => address[]) public validators;
    mapping(address => bool) public isValidator;
    mapping(bytes32 => uint256) public executedTransfers;

    event TransferInitiated(
        bytes32 indexed transferId,
        address indexed sender,
        address indexed receiver,
        address token,
        uint256 amount,
        uint256 destChain
    );
    
    event TransferConfirmed(
        bytes32 indexed transferId,
        address indexed validator
    );
    
    event TransferExecuted(
        bytes32 indexed transferId,
        address indexed receiver,
        uint256 amount
    );

    constructor(uint256 _chainId) {
        chainId = _chainId;
    }

    function initiateTransfer(
        address token,
        uint256 amount,
        uint256 destChain,
        address receiver
    ) external payable returns (bytes32) {
        require(amount >= minTransferAmount, "TigerBridge: BELOW_MIN");
        require(amount <= maxTransferAmount, "TigerBridge: ABOVE_MAX");
        require(msg.value >= GAS_RESERVE, "TigerBridge: INSUFFICIENT_GAS");

        if (token != address(0)) {
            IERC20(token).transferFrom(msg.sender, address(this), amount);
        }

        bytes32 transferId = keccak256(abi.encodePacked(
            msg.sender,
            token,
            amount,
            destChain,
            block.timestamp,
            nonce()
        ));

        Transfer storage transfer = transfers[transferId];
        transfer.sender = msg.sender;
        transfer.receiver = receiver;
        transfer.token = token;
        transfer.amount = amount;
        transfer.destChain = destChain;
        transfer.nonce = nonce();
        transfer.status = Status.Pending;
        transfer.hash = transferId;

        emit TransferInitiated(transferId, msg.sender, receiver, token, amount, destChain);
        return transferId;
    }

    function confirmTransfer(bytes32 transferId) external {
        require(isValidator[msg.sender], "TigerBridge: NOT_VALIDATOR");
        require(transfers[transferId].status == Status.Pending, "TigerBridge: NOT_PENDING");

        Transfer storage transfer = transfers[transferId];
        require(!transfer.hasConfirmed[msg.sender], "TigerBridge: ALREADY_CONFIRMED");

        transfer.hasConfirmed[msg.sender] = true;
        transfer.confirmations++;

        if (transfer.confirmations >= requiredConfirmations) {
            transfer.status = Status.Confirmed;
        }

        emit TransferConfirmed(transferId, msg.sender);
    }

    function executeTransfer(bytes32 transferId) external {
        Transfer storage transfer = transfers[transferId];
        require(transfer.status == Status.Confirmed, "TigerBridge: NOT_CONFIRMED");
        require(executedTransfers[transferId] == 0, "TigerBridge: ALREADY_EXECUTED");

        executedTransfers[transferId] = block.timestamp;
        transfer.status = Status.Executed;

        uint256 fee = (transfer.amount * feePercentage) / 10000;
        uint256 amountToSend = transfer.amount - fee;

        if (transfer.token == address(0)) {
            payable(transfer.receiver).transfer(amountToSend);
        } else {
            IERC20(transfer.token).transfer(transfer.receiver, amountToSend);
        }

        emit TransferExecuted(transferId, transfer.receiver, amountToSend);
    }

    function addValidator(address validator) external {
        require(!isValidator[validator], "TigerBridge: ALREADY_VALIDATOR");
        isValidator[validator] = true;
        validators[chainId].push(validator);
    }

    function removeValidator(address validator) external {
        require(isValidator[validator], "TigerBridge: NOT_VALIDATOR");
        isValidator[validator] = false;
    }

    function setRequiredConfirmations(uint256 count) external {
        requiredConfirmations = count;
    }

    function setFeePercentage(uint256 fee) external {
        require(fee <= 100, "TigerBridge: FEE_TOO_HIGH");
        feePercentage = fee;
    }

    function nonce() public returns (uint256) {
        return transfers[keccak256(abi.encodePacked("nonce"))].nonce;
    }

    receive() external payable {}
}