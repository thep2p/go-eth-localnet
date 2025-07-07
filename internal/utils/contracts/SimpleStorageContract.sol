// SPDX-License-Identifier: MIT
pragma solidity ^0.8.30;

contract Storage {
    uint256 public value;

    event ValueChanged(uint256 newValue);

    function set(uint256 v) public {
        value = v;
        emit ValueChanged(v);
    }
}
