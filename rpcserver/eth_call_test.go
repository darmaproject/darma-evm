// Copyright 2018-2020 Darma Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package rpcserver

import (
	"github.com/intel-go/fastjson"
	"testing"
)

func Test_EthWeb3JsRpcHandler_eth_call_ServeJSONRPC(t *testing.T) {
	// strData := `[{"to":"0x11f4d0a3c12e86b4b5f39b213f7e19d048276dae","data":"0xc6888fa10000000000000000000000000000000000000000000000000000000000000003"},"latest"]`
	// data := []byte(strData);
	// params := fastjson.RawMessage{}
	// params = make([]byte,len(data))
	// copy(params[:],data[:])

	// var call EthWeb3JsRpcHandler_eth_call
	// _,err := call.ServeJSONRPC(nil,&params)
	// if err != nil {
		// t.Errorf("call error: {%s}",err.Error())
	// }
}