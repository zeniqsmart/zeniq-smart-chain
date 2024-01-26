const TestSEP109 = artifacts.require("TestSEP109");

contract("TestSEP109", async (accounts) => {

    it('verify', async () => {
        let blockNum = await web3.eth.getBlockNumber();
        if (blockNum < 3088100) {
            console.log('VRF not enabled!');
            return;
        }

        const testSEP109 = await TestSEP109.new();

        assert.equal(await testSEP109.verify(
            "0x000000000000000000000000000000000000000000000000000073616d706c65",
            "0x032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645",
            "0x027d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
            "0xb250afcc8b62868bf1a971afab5734894faa6c8077bba8e0b5bf9e71abca6d02"
        ), true);

        assert.equal(await testSEP109.verify(
   /* -> */ "0xFF0000000000000000000000000000000000000000000000000073616d706c65",
            "0x032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645",
            "0x027d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
            "0xb250afcc8b62868bf1a971afab5734894faa6c8077bba8e0b5bf9e71abca6d02"
        ), false);
        assert.equal(await testSEP109.verify(
            "0x000000000000000000000000000000000000000000000000000073616d706c65",
   /* -> */ "0xFF2c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645",
            "0x027d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
            "0xb250afcc8b62868bf1a971afab5734894faa6c8077bba8e0b5bf9e71abca6d02"
        ), false);
        assert.equal(await testSEP109.verify(
            "0x000000000000000000000000000000000000000000000000000073616d706c65",
            "0x032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645",
   /* -> */ "0xFF7d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
            "0xb250afcc8b62868bf1a971afab5734894faa6c8077bba8e0b5bf9e71abca6d02"
        ), false);
        assert.equal(await testSEP109.verify(
            "0x000000000000000000000000000000000000000000000000000073616d706c65",
            "0x032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645",
            "0x027d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
   /* -> */ "0xFF50afcc8b62868bf1a971afab5734894faa6c8077bba8e0b5bf9e71abca6d02"
        ), false);
    });

});

contract("SEP109", async (accounts) => {

    it('verify', async () => {
        let blockNum = await web3.eth.getBlockNumber();
        if (blockNum < 3088100) {
            console.log('VRF not enabled!');
            return;
        }

        await web3.eth.sendTransaction({
            from    : accounts[0],
            // to      : "0x0000000000000000000000000000000000002713", // sep109 was BCH
            to      : "0x000000000000000000000000005a454e49510004",    // sep109 ZENIQ live2022 but reachable only after CCRPCForkBlock
            data    : "000000000000000000000000000000000000000000000000000073616d706c65032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645027d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
            value   : 0,
            gasPrice: 10000000000,
        });

        const beta = await web3.eth.call({
            from    : accounts[0],
            // to      : "0x0000000000000000000000000000000000002713", // sep109 was BCH
            to      : "0x000000000000000000000000005a454e49510004",    // sep109 ZENIQ live2022 but reachable only after CCRPCForkBlock
            data    : "000000000000000000000000000000000000000000000000000073616d706c65032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645027d4501b9a66bcdd3fb1833da800859fb98e6463221b78c7ba6cc45f60a582571c9a559df4c03a9b300e3631a5dd20ac90e73f0b3d50d43cad34d6415226a0503d8565661a9714cb29a0c5088227c426f",
            value   : 0,
            gasPrice: 10000000000,
        });
        assert.equal(beta, '0xb250afcc8b62868bf1a971afab5734894faa6c8077bba8e0b5bf9e71abca6d02');
    });

});
