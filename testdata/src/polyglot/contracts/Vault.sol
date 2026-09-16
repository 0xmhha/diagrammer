// SPDX-License-Identifier: MIT
pragma solidity 0.8.36;

/// Holds deposits for one owner.
contract Vault {
    /// Emitted whenever a deposit lands.
    event Deposited(address from, uint256 amount);

    /// Take a deposit and record it.
    function deposit(uint256 amount) external {
        _record(amount);
    }

    /// Internal bookkeeping.
    function _record(uint256 amount) internal {}
}
