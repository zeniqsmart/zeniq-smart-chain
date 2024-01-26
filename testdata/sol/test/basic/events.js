const EventEmitter = artifacts.require("EventEmitter");

contract("EventEmitter", async (accounts) => {

    it('emit event 1', async () => {
        const emitter = await EventEmitter.new({ from: accounts[0] });
        const res = await emitter.emitEvent1();
        const log = res.logs.find(element => element.event.match('Event1'));
        assert.strictEqual(log.args.addr, accounts[0]);
    });

    it('emit event 2', async () => {
        const emitter = await EventEmitter.new({ from: accounts[0] });
        const res = await emitter.emitEvent2(123);
        const log = res.logs.find(element => element.event.match('Event2'));
        assert.strictEqual(log.args.addr, accounts[0]);
        assert.strictEqual(log.args.value.toString(), '123');
    });

    it('emit event then revert', async () => {
        const emitter = await EventEmitter.new({ from: accounts[0] });
        try {
            await emitter.emitEvent1ThenRevert();
            throw null;
        } catch (error) {
            assert(error, "Expected an error but did not get one");
        }
    });

    // it('emit event then revert: web3', async () => {
    //     const emitter = await EventEmitter.new({ from: accounts[0] });
    //     const result = await web3.eth.sendTransaction({
    //         from: accounts[0],
    //         to  : emitter.address,
    //         data: "0a132714",
    //     });
    //     console.log(result);
    // });

});
