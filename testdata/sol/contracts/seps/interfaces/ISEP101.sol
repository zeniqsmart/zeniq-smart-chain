// SPDX-License-Identifier: MIT
pragma solidity >=0.8.0;

interface ISEP101 {

    function set(bytes calldata key, bytes calldata value) external;
    function get(bytes calldata key) external view returns (bytes memory);

}