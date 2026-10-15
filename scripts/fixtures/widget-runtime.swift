import Foundation
import JavaScriptCore

@main
struct WidgetRuntimeAcceptance {
  static func main() {
    var allocations = 0
    let runtime = WidgetsJSRuntime(bundleScript: { "function __expoWidgetRender(p,e) { return __expoWidgetLayout(p,e); }" }, createContext: {
      allocations += 1
      return JSContext()
    })
    let layout = "function(p,e) { if(p.fail) throw new Error('expected'); return {owner:p.owner, value:p.value}; }"
    for index in 0..<300 {
      let result = runtime.evaluate(layout: layout, functionName: "__expoWidgetRender", arguments: [["owner": "account-\(index % 2)", "value": index], [:]]) as? [String: Any]
      precondition(result?["owner"] as? String == "account-\(index % 2)")
      precondition(result?["value"] as? Int == index)
    }
    precondition(allocations == 1, "Repeated regions must not allocate new engines")
    precondition(runtime.evaluate(layout: layout, functionName: "__expoWidgetRender", arguments: [["fail": true], [:]]) == nil)
    precondition(runtime.evaluate(layout: "broken syntax", functionName: "__expoWidgetRender", arguments: [[:], [:]]) == nil)
    DispatchQueue.concurrentPerform(iterations: 100) { index in
      let alternate = "function(p,e) { return {value:p.value + 1}; }"
      let result = runtime.evaluate(layout: alternate, functionName: "__expoWidgetRender", arguments: [["value": index], [:]]) as? [String: Any]
      precondition(result?["value"] as? Int == index + 1)
    }
    precondition(allocations == 1)
    var attempts = 0
    let recoverable = WidgetsJSRuntime(bundleScript: {
      attempts += 1
      return attempts == 1 ? "throw new Error('bundle unavailable')" : "function __expoWidgetRender(p,e) { return __expoWidgetLayout(p,e); }"
    })
    precondition(recoverable.evaluate(layout: layout, functionName: "__expoWidgetRender", arguments: [[:], [:]]) == nil)
    let recovered = recoverable.evaluate(layout: layout, functionName: "__expoWidgetRender", arguments: [["owner": "new", "value": 7], [:]]) as? [String: Any]
    precondition(recovered?["value"] as? Int == 7)
  }
}
