package widget

import (
	"fmt"
	"reflect"
	"strings"
)

func FormatElementTree(root Element) string {
	var sb strings.Builder
	sb.WriteString("strict digraph{\n")
	sb.WriteString("rankdir=TB;\n")
	var visit func(parent Element, el Element)
	visit = func(parent Element, el Element) {
		w := el.Handle().widget
		sb.WriteString("{\nrank=same;\n")
		fmt.Fprintf(&sb, "n%[1]p [label=\"%[1]T\", fillcolor=lightgreen, style=filled];\n", w)
		fmt.Fprintf(&sb, "n%[1]p [label=\"%[1]T (%s)\", fillcolor=magenta, style=filled];\n", el, el.Handle().lifecycleState)
		if el, ok := el.(RenderObjectElement); ok {
			obj := el.RenderHandle().RenderObject
			if obj != nil {
				fmt.Fprintf(&sb, "n%[1]p [label=\"%[1]T\", fillcolor=cyan, style=filled];\n", obj)
			}
		}

		if state := reflect.ValueOf(el).Elem().FieldByName("State"); state.IsValid() {
			statei := state.Interface()
			if statei != nil {
				fmt.Fprintf(&sb, "n%[1]p [label=\"%[1]T\", fillcolor=yellow, style=filled];\n", statei)
				fmt.Fprintf(&sb, "n%p -> n%p [color=yellow];\n", el, statei)
			}
		}

		sb.WriteString("}\n")

		fmt.Fprintf(&sb, "n%p -> n%p [color=lightgreen];\n", w, el)

		if parent != nil {
			parentW := parent.Handle().widget
			fmt.Fprintf(&sb, "n%p -> n%p;\n", parentW, w)
		}

		if el, ok := el.(RenderObjectElement); ok {
			obj := el.RenderHandle().RenderObject
			if obj == nil {
				fmt.Fprintf(&sb, "n%p -> NIL_RENDER_OBJECT [color=magenta];\n", el)
			} else {
				fmt.Fprintf(&sb, "n%p -> n%p [color=magenta];\n", el, obj)

				if objp := obj.Handle().Parent; objp != nil {
					fmt.Fprintf(&sb, "n%p -> n%p;\n", objp, obj)
				}
			}
		}
		VisitChildren(el, func(child Element) bool {
			visit(el, child)
			fmt.Fprintf(&sb, "n%p -> n%p;\n", el, child)
			return true
		})
	}
	visit(nil, root)

	sb.WriteString("}\n")

	return sb.String()
}
