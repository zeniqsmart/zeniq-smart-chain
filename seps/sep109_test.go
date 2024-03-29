package seps

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/btcsuite/btcd/btcec"
	"github.com/holiman/uint256"
	"github.com/vechain/go-ecvrf"

	gethcmn "github.com/ethereum/go-ethereum/common"
	"github.com/zeniqsmart/zeniq-smart-chain/internal/testutils"
)

var (
	//vrfAddr = gethcmn.HexToAddress("0x0000000000000000000000000000000000002713") // sep109 was BCH
	vrfAddr = gethcmn.HexToAddress("0x000000000000000000000000005a454e49510004") // sep109 ZENIQ live2022 but reachable only after CCRPCForkBlock
)

// https://github.com/vechain/go-ecvrf/blob/master/tests/secp256_k1_sha256_tai.json
const vrfTestCasesJSON = `
[
  {
      "beta": "612065e309e937ef46c2ef04d5886b9c6efd2991ac484ec64a9b014366fc5d81",
      "alpha": "73616d706c65",
      "pi": "031f4dbca087a1972d04a07a779b7df1caa99e0f5db2aa21f3aecc4f9e10e85d08748c9fbe6b95d17359707bfb8e8ab0c93ba0c515333adcb8b64f372c535e115ccf66ebf5abe6fadb01b5efb37c0a0ec9",
      "sk": "c9afa9d845ba75166b5c215767b1d6934e50c3db36e89b127b8a622b120f6721",
      "pk": "032c8c31fc9f990c6b55e3865a184a4ce50e09481f2eaeb3e60ec1cea13a6ae645"
  },
  {
      "beta": "00acd42d48046e13552f54919286c2085aec6fb874854d036f66ad572c99e7ab",
      "alpha": "73616d706c65",
      "pi": "029a2df6ca1d5f734945fb6847669f839eb9ecf127fa8314e5a6a5c4695c3f4d159009b3741cdec6b0d7c70e3aae6b82aeb1aad555499bd6ce10b35fa230079e6fa752e8d4755ffd285aef5133dad7a64b",
      "sk": "01",
      "pk": "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
  },
  {
      "beta": "c355718640883112731fce0b5dd97c34492d226280654dcf0ada1d6b32e3384b",
      "alpha": "73616d706c65",
      "pi": "0205c5a6ed80f7ffbf9f47583e873717e86c8405349266745a0504ea7ca68876ce6afe0e3cccfc7bba83a6d16771d80e26a46ad25be631869d6f60a34c12a19b868815182657288f57afb91166ceed3cc5",
      "sk": "02",
      "pk": "02c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
  },
  {
      "beta": "09a93f6e8a6037db146d862eec33f56c8053bb4fda309f8ecbcbe04592aabd37",
      "alpha": "73616d706c65",
      "pi": "02a14b92076becc501b9ac761c18cacd792e0b30ad2b6907e1273dbe3762a9d29cf97fe3a904d2123ef98030a929ea91b40f1d5f78d9eb09c85ee2caa962b17de41abd4cf6be036e90c9826ae4e6e3673a",
      "sk": "03",
      "pk": "02f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"
  },
  {
      "beta": "5a0bce08f650d5ef80b0254ee814360e1f2c944a6c2b15e3171374f362cd92b4",
      "alpha": "73616d706c65",
      "pi": "03a3ca50b6f1873b8ca55348d0c62b0a2fd774aa9b96d1061c0917d6f9da5fe560dcddbda349d2db15751c965baf88ddb67075b5a441400c5ef85d9dffd6a16833993af6f05901c2421210d72d114c4ba0",
      "sk": "04",
      "pk": "02e493dbf1c10d80f3581e4904930b1404cc6c13900ee0758474fa94abe8c4cd13"
  },
  {
      "beta": "e06dc093da223b40225633628c94a0bf909a3bb19a2f838932b4627f740354a1",
      "alpha": "73616d706c65",
      "pi": "030b4806f7f7398ba03951990740243d3f84a0815d85e2be439ee42bd8f249bd44e56800354296f9c05d30e06966009baf98c963028070217f28c0a298f9ecd56726bbc7c4faf1c62691972bfbc9fbd684",
      "sk": "05",
      "pk": "022f8bde4d1a07209355b4a7250a5c5128e88b84bddc619ab7cba8d569b240efe4"
  },
  {
      "beta": "811a4fe231758d65b9b20d436a19256765776c812966032b16ce2b21e5c69be7",
      "alpha": "73616d706c65",
      "pi": "02f0fdc752a0c63cae4d33f65af4167a7a8b04711d840230df0d211ee3d6e2a2ad66625f4d9db7798ab664bb2da77736db6c9b7828cd8310a2e5f677224e1676afbb731cf92018a49c0452d1080aac5f3a",
      "sk": "06",
      "pk": "03fff97bd5755eeea420453a14355235d382f6472f8568a18b2f057a1460297556"
  },
  {
      "beta": "0b772c54b2125194bd26b7e3ce2c4606043f65d99d3c65aef6e3b61dffd20aae",
      "alpha": "73616d706c65",
      "pi": "036e041791df3880003115548bf1c491700133a23fb6c1edbef6a23f7dd67c05d5e227f496df589bdd68ffc28fb00246681abb9399a4207fbf42c2a64fa47615b4a64dc25cf0f6bc368353bfb25f63c95b",
      "sk": "07",
      "pk": "025cbdf0646e5db4eaa398f365f2ea7a0e3d419b7e0330e39ce92bddedcac4f9bc"
  },
  {
      "beta": "8f7729e4288e7630b358a6cef560479f0e441926ae0fa6a3a96f17b63b3d1fb8",
      "alpha": "73616d706c65",
      "pi": "02b5a04c6340c0d09b26da8f75c7713ed871e9f673bb87c00ee0e76e8045faad9ece28cfdbc41eab04205963c3252b7229d94793491e4a7f48e09c2d85b3ee2a1131a27b670b07c016dc00e4d8f3daa242",
      "sk": "08",
      "pk": "022f01e5e15cca351daff3843fb70f3c2f0a1bdd05e5af888a67784ef3e10a2a01"
  },
  {
      "beta": "ce465b7e061bae415e10d8aafd32d92e2f499d959632fe5983cb0b2d4ee7b3b5",
      "alpha": "73616d706c65",
      "pi": "02cf1639991c8eb219993f1b9921a413e2ebc72d7b533ab952c92188b9b4ee2bd8cd5417a75c80d3022a3719297b2699b958d27e5e9fe03c8fc96766234fa497bee8287a76f6db22ff482cec9505e51a5f",
      "sk": "09",
      "pk": "03acd484e2f0c7f65309ad178a9f559abde09796974c57e714c35f110dfc27ccbe"
  },
  {
      "beta": "941b5f76d95772045e824e30d85106a4a3b5920b901e178d3f62ec8e473ccf69",
      "alpha": "73616d706c65",
      "pi": "034e371e26303b9deb6f650f8faee60a2673b2bfef5b5a9af066aca5e7c12695becd27149218141ceb7f7e58da293679388c3b7b5109ec360a1f50decb1fa89c11b19476f5b2786ecba75f3642d942ba31",
      "sk": "0a",
      "pk": "03a0434d9e47f3c86235477c7b1ae6ae5d3442d49b1943c2b752a68e2a47e247c7"
  },
  {
      "beta": "79403a19944c3516d102cfa42cd5dd3f16c5c9bd457bf0659e2305374af3dccd",
      "alpha": "73616d706c65",
      "pi": "0384e3011d9e8235de77c211c0e57b0ed4bbffbe4b1d1a946278d13943be6c4280206842bfc58d564262a3214cfab3098d2887923eb659ff543aaa9c48d821acf1198bde5d824d27e9a9a6c2f84c6bdfdb",
      "sk": "0b",
      "pk": "03774ae7f858a9411e5ef4246b70c65aac5649980be5c17891bbec17895da008cb"
  },
  {
      "beta": "746ca26b7c90df37d309ecce42757e6bf9423ad31211a02e8d9d86aea72f2f3a",
      "alpha": "73616d706c65",
      "pi": "03ad49eb72e2c7789d851c7bf2a3137cee17ab304ae7acb7b22739c5aae48eb339199381d85ae4654b2f5488499b4d3a13d6055ca740de70bb6d9b2c0cb962d873e13cd1d1d1c0c4eef9f9809f88e36fc0",
      "sk": "0c",
      "pk": "03d01115d548e7561b15c38f004d734633687cf4419620095bc5b0f47070afe85a"
  },
  {
      "beta": "51096a3de69492e852477271a77a5b95de5a0cc2fc43128177b9a85e3184318d",
      "alpha": "73616d706c65",
      "pi": "038be9f8885740d6a336f0ec8249ec010f99fb4f0e42f09804c6f98f0be907327419f8249ebc3fb6fb372402893c55422987d9d36f6dfbdc7cfa88fb75dca13b095f412a1d82b358b1d17406991af576e4",
      "sk": "0d",
      "pk": "03f28773c2d975288bc7d1d205c3748651b075fbc6610e58cddeeddf8f19405aa8"
  },
  {
      "beta": "b3a048bc1682c3d5ba4d79a951b38f6d9729a328afb38fdd22c5f17247eeb642",
      "alpha": "73616d706c65",
      "pi": "0354a5641699f62565bec88e75ed465c052f655048a2de85ae39f32e968a80faeb6e21ac4919dc0710e1f1a76184988926a2b354c52cf1ca8529157aa5574c385b86def3a76a7cb9e0d0a11227bc651dac",
      "sk": "0e",
      "pk": "03499fdf9e895e719cfd64e67f07d38e3226aa7b63678949e6e49b241a60e823e4"
  },
  {
      "beta": "eb5c23bc8773fa4c83ef3bf88bf63aceffe6b042e220d8826db222dc55f915fe",
      "alpha": "73616d706c65",
      "pi": "0398f53f2ed4d687e6a65eaf7ce4d63e99e2db78a12302c782ffe6012737eb2d10257f7a86288479153dafa74c14092fd5acc60c953f225840ee6c92ec67106a979ead5f8c12142373b119190a012ac780",
      "sk": "0f",
      "pk": "02d7924d4f7d43ea965a465ae3095ff41131e5946f3c85f79e44adbcf8e27e080e"
  },
  {
      "beta": "e96f1bab42d66b32d2fb8ea1e4655808881cd9f208279a0757ef990575e7adc0",
      "alpha": "73616d706c65",
      "pi": "024ca2c0b270a6c632cade38ff097d66de362c54064e847fd96bd4067b71028db4ead3e112b4cc78b826165d6fde084924f58ae74f69c4542d68cdd5a33e03d5bd2e7fc5f5fda4c9e3d157fbb213fddd3d",
      "sk": "10",
      "pk": "03e60fce93b59e9ec53011aabc21c23e97b2a31369b87a5ae9c44ee89e2a6dec0a"
  },
  {
      "beta": "dbfeac68bfc25d1fd3f9d0bfe093edfef9e90c1b4d2940ac961c1d08ba66deb4",
      "alpha": "73616d706c65",
      "pi": "028594a13f389fbb3618bb5db54059ea673f087d7193ed8513d1abc7f3a5228d0e99a976d08666bdb46b287ec41945de29cce12c2dac0ba226ef3ef383a0fba8e138b415816c5243af66ef5a51244d9616",
      "sk": "11",
      "pk": "03defdea4cdb677750a420fee807eacf21eb9898ae79b9768766e4faa04a2d4a34"
  },
  {
      "beta": "87a0ff6de5a282746082ed187abc29f534c1ee4f641e604c7d241fea5155b2a2",
      "alpha": "73616d706c65",
      "pi": "03e9593d98552a6e3c3f45a8a566b4d2de9bd5ff1837b4ca91655dc6e91d59fb5b335fd59c15668b371093ee81df275c876761ebcbd60fa8127c7e601f99ce340a3ae8c00864c0e38ebe749e566c151240",
      "sk": "12",
      "pk": "025601570cb47f238d2b0286db4a990fa0f3ba28d1a319f5e7cf55c2a2444da7cc"
  },
  {
      "beta": "32306c22ad369a6e0fd638f503c0dbf674cd9d66b5da0a8c83c1d78157814f90",
      "alpha": "73616d706c65",
      "pi": "02833a4a802ec7f90b91dc59cfc8138669d2e6ec3f933c4fc7e44389a5e02c6032816f05729899c003ae77025e22c703809f8ee1b35d04d250800c7e90533dc4f29b3667065fc2cae0cc3cd356962ac62a",
      "sk": "13",
      "pk": "022b4ea0a797a443d293ef5cff444f4979f06acfebd7e86d277475656138385b6c"
  },
  {
      "beta": "4e853d2b79f1d2dd8e23be493efa8c685afc30198070aa5acbc53fddcace9aa9",
      "alpha": "73616d706c65",
      "pi": "022d2bb6961468e5a3aadae02157e584a4d45f58121185cafa11b93fae6eb4a4604d03e431646fb1ee06c04e562cbd0e7d8c4fc01bc8dc0ca0f7f5d4e138be2963f6ab25843a12dd6c5e34cfda1ecabf04",
      "sk": "14",
      "pk": "024ce119c96e2fa357200b559b2f7dd5a5f02d5290aff74b03f3e471b273211c97"
  },
  {
      "beta": "642a4819a8e8e5739cc0f4bd5221d641774f2d33b75007c4de61f4f2089df92c",
      "alpha": "73616d706c65",
      "pi": "02570056bade9dc5204721d0ecd39906a72468d027dfa1a896b8f6a88d50fc5526f7afb112bb95a2e13f326cbf1c8de77b7596195575b4dd712edbfd96acafdcbc0fa1dfae4041bb64d8e25b276426ea5a",
      "sk": "018ebbb95eed0e13",
      "pk": "02a90cc3d3f3e146daadfc74ca1372207cb4b725ae708cef713a98edd73d99ef29"
  },
  {
      "beta": "b042e27b4d0e1f1a1c8ec7d4c2c0b2ae988d4cbc526d1ffb11b0087bea97a630",
      "alpha": "73616d706c65",
      "pi": "02b87402d5f26e06c14a3136a6c868316d2a2d413d5ef2ef91cae3625d57e87ea326b2bb5f67e4fe3f5c65df958d1ecd7a9bebea344b1facd08fbc7512701bcababbc39c410d9d1c0980e688feada47468",
      "sk": "159d893d4cdd747246cdca43590e13",
      "pk": "03e5a2636bcfd412ebf36ec45b19bfb68a1bc5f8632e678132b885f7df99c5e9b3"
  },
  {
      "beta": "3a36fc2ff539e516897c53d61951209dcac171500ed79692434a28e0cb9c3272",
      "alpha": "73616d706c65",
      "pi": "02e111fc96dd022c02f45df180c3707d5a48a5eb669aad2f7b45345b5f95a38f68b82b8cfa5dee906023fe4c2a15723636e29c8fc8d65ade1620b0d6453331f0237f632e89f0352fd25662d5630b3a6034",
      "sk": "3fffffffffffffffffffffffffffffffaeabb739abd2280eeff497a3340d9050",
      "pk": "03a6b594b38fb3e77c6edf78161fade2041f4e09fd8497db776e546c41567feb3c"
  },
  {
      "beta": "c6e3b662984301fc84c5eb2f5c0f435aee2975a731e6707bb9e50113e4bc2809",
      "alpha": "73616d706c65",
      "pi": "023a435fc5fab74b0b33eeb7c62447efc323e6e33a19657e7a0a473451b885fe841f28b91f43834ab659f26ea94d9dbca325192c45589afc1415e508b72247c64e385d3f9aa13c2e571d252335b8f63a3e",
      "sk": "7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0",
      "pk": "0300000000000000000000003b78ce563f89a0ed9414f5aa28ad0d96d6795f9c63"
  },
  {
      "beta": "a25353782f363555d90c822a347151272d364103aa49513105bcc247287ff6a9",
      "alpha": "73616d706c65",
      "pi": "02f769ca0cb1a96046265c19d5ef44deade0eb42a6aab094c7fde5bcdae09455db2ba3ffa7e72d01ead1fedc0162507c5d8ccbde61e09cd533b807404b75a4acb8573881f86abbf41bc0877acb3362604f",
      "sk": "bfffffffffffffffffffffffffffffff0c0325ad0376782ccfddc6e99c28b0f0",
      "pk": "02e24ce4beee294aa6350faa67512b99d388693ae4e7f53d19882a6ea169fc1ce1"
  },
  {
      "beta": "9731f862d34587fb91521d785a30ff57188c57efd4b55239199ae4ed31bcebf8",
      "alpha": "73616d706c65",
      "pi": "02d196a5eb787d5d8a33247607d78d5a48164ac4d899dba33cb3a5f68032124ba1b2221b96ae997d450ddd8e5863a0cc7cf1b3f50431aa6031b6807396edfab5d1f5812c7e8d40f157028afb7aa51a41d5",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036412d",
      "pk": "034ce119c96e2fa357200b559b2f7dd5a5f02d5290aff74b03f3e471b273211c97"
  },
  {
      "beta": "54c8f3e98a8a8e5b77d327cdb7a71e5959996b5619972a8b472d7a3799a79387",
      "alpha": "73616d706c65",
      "pi": "0391aeaecb887e9d6f0c0f64d20f28acbe687bffabaaea0ff5237f236693eb5b56c88099425415cd9d242c70249f7abb8958a327ad341b4fca735f8ba61b57c9735252b72afab7e0cb6e49c9436366e800",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036412e",
      "pk": "032b4ea0a797a443d293ef5cff444f4979f06acfebd7e86d277475656138385b6c"
  },
  {
      "beta": "f9fba571cb27776c07d2bc42d670952e1965357942eca3edb5f80e28bc9aaed0",
      "alpha": "73616d706c65",
      "pi": "03b9ffede3d97b9753f8f31cd5d56442c525a5bccc2de1fc547886ee08bca9b4f3c1d44da0826ddfd763801c42875d41deeae99422c0e9e6a97e07e2689f58289bffe499069785dcffa4ff93a5b1e15856",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036412f",
      "pk": "035601570cb47f238d2b0286db4a990fa0f3ba28d1a319f5e7cf55c2a2444da7cc"
  },
  {
      "beta": "c66931ad96cb4e1a44202fcd7882089b1cf77b07c426d292e0e15deca5c1b027",
      "alpha": "73616d706c65",
      "pi": "03ca01e6d80f99bd12c5c00142a9eb0c0e029f999a1e945a70110c944d5d5981c9814fb051e88c36f0c14d9acdfc3040b37fcc77fa1bfc23466730a108849e0a08f5e9c79331b04803e568ea4b553ce3f1",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364130",
      "pk": "02defdea4cdb677750a420fee807eacf21eb9898ae79b9768766e4faa04a2d4a34"
  },
  {
      "beta": "4b00d6d00864c7972105755b538d5f62a3585b6e8e7061fd107317fa1004efc0",
      "alpha": "73616d706c65",
      "pi": "03df963611501cf382e2730131618377ab38486f483db1eab7feb6ade0e1b0141bd3d291b7e45a1b94cabbafa5fca3fb7ba36b158bbcdb2292383689a6231e201a3f78f9f40757e99f80e8032adbc8d4ce",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364131",
      "pk": "02e60fce93b59e9ec53011aabc21c23e97b2a31369b87a5ae9c44ee89e2a6dec0a"
  },
  {
      "beta": "e1e9b8491278b6faf79d433cee7d9b01256f18b3d63601f6231332ce3751411c",
      "alpha": "73616d706c65",
      "pi": "03ee58341c2222f7671318eff4bf2bd5588221d37d133a8aafcff5162d56af906581ce8cc5d45d546b8cf2c6d22026b934688e9a68555d4386d75f9b9e554b58b886ad91285872a2a6f137576b6bf9513f",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364132",
      "pk": "03d7924d4f7d43ea965a465ae3095ff41131e5946f3c85f79e44adbcf8e27e080e"
  },
  {
      "beta": "72c8e10530d3f0c6e452f8f20d911908eb01887c62bbae0b1eb35cb1f36b7985",
      "alpha": "73616d706c65",
      "pi": "03ae19c4bac9d64009b7dabf9095c3ee3c848249269d41d5ee492683cef4a0b8464fa567b84a2bfe1c7359696522d01e083defdf2c4fe5aaad7bf67c93aab74d23069d05419c59c5cd5daed14e63bdc26f",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364133",
      "pk": "02499fdf9e895e719cfd64e67f07d38e3226aa7b63678949e6e49b241a60e823e4"
  },
  {
      "beta": "11fee8e23d484d9aa8ed151f8452be11e70cfad8a44f707c00b04a11270c3d7a",
      "alpha": "73616d706c65",
      "pi": "0261cb37ca1f9c0ee11e41aadf4637fdddccb3f70f8ff1903727fbc2bd220720e737048df6aa7ddec95dc6a5f93f6808b5bbf2983c1733ce7b686dddc457001dbbae277d251d61f69e1af22d69b60c975e",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364134",
      "pk": "02f28773c2d975288bc7d1d205c3748651b075fbc6610e58cddeeddf8f19405aa8"
  },
  {
      "beta": "dfa3543db662a08aac90bfd7dd9b39b77dacf16dda462fe5eaa26b0f595fc0f5",
      "alpha": "73616d706c65",
      "pi": "0376661cbff92aae582298a7348f4d8f7834e2d8f6707c9706f52e65aacf968d80b24c972d16acca689cf66a1d100c26d2b141b1c8b9835e6710db5126284b9540f43cf31394f9b3e0a87449df7ef6aee6",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364135",
      "pk": "02d01115d548e7561b15c38f004d734633687cf4419620095bc5b0f47070afe85a"
  },
  {
      "beta": "caae0b8dd19e20fd52f43b2fd416228b46ad625aa68ef6424ff388fb4727e0dc",
      "alpha": "73616d706c65",
      "pi": "0204fa576f63771c34e6cdb98f59997584528d109c7592ab867374d9b91051a4d1e875685ea35673d901ff06f18d7e89bc098be8762abf7688dba945d09d9b71348624b40c4b903de2bd3cc44abb2fafc7",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364136",
      "pk": "02774ae7f858a9411e5ef4246b70c65aac5649980be5c17891bbec17895da008cb"
  },
  {
      "beta": "97e0785b78305d909af2a255e1b26d4faf5879d4e7640bceac71e56b3851bec0",
      "alpha": "73616d706c65",
      "pi": "024d4ff3ad7689b905b5c4be9de0bd8d7960e30f145903fe715af943852229f269122fadc5f835ee029d306ad7d90f5c6011ad67d24a327cccf3f39018e34df7a1544d6748755fa07ce2013729816e2330",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364137",
      "pk": "02a0434d9e47f3c86235477c7b1ae6ae5d3442d49b1943c2b752a68e2a47e247c7"
  },
  {
      "beta": "e0a3519f3dd1597039b5617d5b09c8ed5c723d1589010c1d6284dd8fb9d5ea7b",
      "alpha": "73616d706c65",
      "pi": "022eb72eccc7228307eaf7946a28feac02de8223534800cc71d7d1195fb0a7c88630d9be168212ecee12897644c456c22eaa9be58c9c8bb92a86a73e787f44f55a57203ddd9d80bf4e1ad4ac21c03a5f2c",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364138",
      "pk": "02acd484e2f0c7f65309ad178a9f559abde09796974c57e714c35f110dfc27ccbe"
  },
  {
      "beta": "ba9235a6d8c3a2efa2b6cc2d8f23d3b9169ae0a0363db2192465aefbff07ed09",
      "alpha": "73616d706c65",
      "pi": "038fbe2d674ae973b17ee720413e94ee0387d1f794766ec9649d97ccbf6afaa22ce90783ffc7eb0c082db401cb81203ced6223bf24f66cd2c19351f4e18c9fd7a2884d82870ea357bd5bbb4ce9e1e4e840",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364139",
      "pk": "032f01e5e15cca351daff3843fb70f3c2f0a1bdd05e5af888a67784ef3e10a2a01"
  },
  {
      "beta": "9c9dc6b4b61b59950c15c35fb665cc94879b0297fac4edf2803de529b00f8c0c",
      "alpha": "73616d706c65",
      "pi": "0235731391a2ed6ff06cdb279b71ae0151d4f43041cfa8c27d958ab95d08b1bcae807694c56c2ec4ebc855253e3a66e798ea6701ae6861a2ad67a8c2a3ce14c3e8af2f3936fabd5a13dde062afd040e50a",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413a",
      "pk": "035cbdf0646e5db4eaa398f365f2ea7a0e3d419b7e0330e39ce92bddedcac4f9bc"
  },
  {
      "beta": "110f1fa93881cf624c22b72f1b79e6138a052a462ff10d7aa56f501835a8f6b8",
      "alpha": "73616d706c65",
      "pi": "03913bdfa315ab0963b03c34ebe265751e8b5904837bbda75629423b485924fee45b397e9d697239fdcfd435f1b21082d94f1917c792300c4ada68c0ac4e9da7cf2db99c5cbf2775ac4161ea2ed3c589a6",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413b",
      "pk": "02fff97bd5755eeea420453a14355235d382f6472f8568a18b2f057a1460297556"
  },
  {
      "beta": "a43ebaef2262310b95e140287c861c53edefc13c37696c3f89234d1a45eccf17",
      "alpha": "73616d706c65",
      "pi": "024dbc319514312b5544e6b587a978dfccbdc862d7fc33c5dda706efb569613df99fa8b04b237169cb92397564f92bd1e45e041817e7a3368fa1f47fcca1e9bd01a6915e645b2d411815b2b95c9efef590",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413c",
      "pk": "032f8bde4d1a07209355b4a7250a5c5128e88b84bddc619ab7cba8d569b240efe4"
  },
  {
      "beta": "c8952a9439d26d3e761399de1fd734a2338c15893ece5a3efc72e25c9f007da8",
      "alpha": "73616d706c65",
      "pi": "03e30118c907034baf1456063bf7b423972e13e1743bf8dbb2e00fd8ba4a8c367a299bc3859123464d87fd4a508e5a1321f6bde3f00104b2c8af1769781dfb02e749946f4e17f6e429be0f4e4e3085e320",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413d",
      "pk": "03e493dbf1c10d80f3581e4904930b1404cc6c13900ee0758474fa94abe8c4cd13"
  },
  {
      "beta": "25daded1cb7561c8e0013315a6f6d9dd1611d95c92caf5f920bc437ae0180a55",
      "alpha": "73616d706c65",
      "pi": "02ed1bb54a9092c8fd50ae8cea3322e127600a0e32840d9bc4664cfab08b1c6ba3a36ad7913367088f6e6cdbc91a061cfbd0fab0093344414aa16e43dcc5394c7a06cb46afb049eeffc2e99d16992bc228",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413e",
      "pk": "03f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"
  },
  {
      "beta": "3bcee6576d82d011563480c8fcc5751ba6aea58313dbba4cb278d2f74eaee3ea",
      "alpha": "73616d706c65",
      "pi": "03359425334b14173856433b4e695f1d19c7c0cb4eb9b5c72b0b00afe170ce7fd738334a976a8be4582b05a480cdecf8a4f4dd9d0694ae1dcb384429a1c99082bdb2845e2c3010054071489f41fc4b65a5",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413f",
      "pk": "03c6047f9441ed7d6d3045406e95c07cd85c778e4b8cef3ca7abac09b95c709ee5"
  },
  {
      "beta": "8efc7ce3fa0ee91c2e7d45ead94883776e37bdb3af67f386e7ec500e76e066dd",
      "alpha": "73616d706c65",
      "pi": "03cc27d840191d06dfca94d9346cc5b85830dcf9c9e7e4a41cc857d841bd48186c6c5d463591b9632297b3aab781d23263fd9cfc41fbb6affa02840bb903f51b494dc492087ba6fe04acf7c5b54ec0de24",
      "sk": "fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140",
      "pk": "0379be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
  }
]
`

type TestCase struct {
	Sk    string `json:"sk"`
	Pk    string `json:"pk"`
	Alpha string `json:"alpha"`
	Pi    string `json:"pi"`
	Beta  string `json:"beta"`
}

func TestVRF(t *testing.T) {
	hardFork1Height := int64(math.MaxInt64 - 999) // beyond CCRPCForkBlock from DefaultAppConfigWithHome
	key1, addr1 := testutils.GenKeyAndAddr()
	_app := testutils.CreateTestAppWithArgs(testutils.TestAppInitArgs{
		StartHeight: &hardFork1Height,
		PrivKeys:    []string{key1},
	}, nil)
	defer _app.Destroy()

	var testCases []TestCase
	err := json.Unmarshal([]byte(vrfTestCasesJSON), &testCases)
	require.NoError(t, err)

	for _, testCase := range testCases {
		skBytes := testutils.HexToBytes(testCase.Sk)
		sk, _ := btcec.PrivKeyFromBytes(btcec.S256(), skBytes)

		alpha, err := uint256.FromHex("0x" + testCase.Alpha)
		alphaBytes := alpha.PaddedBytes(32)
		require.NoError(t, err)

		betaBytes, piBytes, err := ecvrf.Secp256k1Sha256Tai.Prove(sk.ToECDSA(), alphaBytes)
		require.NoError(t, err)

		inputData := testutils.JoinBytes(alphaBytes, testutils.HexToBytes(testCase.Pk), piBytes)
		//println(hex.EncodeToString(inputData))
		status, statusStr, outputData := _app.Call(addr1, vrfAddr, inputData)
		//println(status, statusStr, hex.EncodeToString(outputData))
		require.Equal(t, 0, status)
		require.Equal(t, "success", statusStr)
		require.Equal(t, betaBytes, outputData)
	}

}
