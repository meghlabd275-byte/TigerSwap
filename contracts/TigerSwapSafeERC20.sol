// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/// @title TigerSwapSafeERC20
/// @notice Call-based ERC-20 transfer helpers.
///
/// A direct `IERC20(token).transfer(...)` reverts against a token that returns
/// no data (USDT on Ethereum does exactly that) and treats a silent `false`
/// return as success. These helpers accept a zero-length return and reject an
/// explicit `false`, so every transfer either moves the funds or reverts.
library TigerSwapSafeERC20 {
    /// @dev Returns true on success so the call can sit inside a `require`.
    function safeTransfer(address token, address to, uint256 value) internal returns (bool) {
        (bool ok, bytes memory data) = token.call(
            abi.encodeWithSelector(IERC20.transfer.selector, to, value)
        );
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: TRANSFER_FAILED");
        return true;
    }

    function safeTransferFrom(address token, address from, address to, uint256 value) internal returns (bool) {
        (bool ok, bytes memory data) = token.call(
            abi.encodeWithSelector(IERC20.transferFrom.selector, from, to, value)
        );
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: TRANSFER_FAILED");
        return true;
    }

    function safeApprove(address token, address spender, uint256 value) internal returns (bool) {
        (bool ok, bytes memory data) = token.call(
            abi.encodeWithSelector(IERC20.approve.selector, spender, value)
        );
        require(ok && (data.length == 0 || abi.decode(data, (bool))), "TigerSwap: APPROVE_FAILED");
        return true;
    }
}
