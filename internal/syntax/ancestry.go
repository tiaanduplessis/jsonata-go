package syntax

// The parent operator (%) is resolved at compile time in jsonata-js. Each %
// creates a slot that asks for an ancestor one level up. The slot travels up
// the expression until a path resolves it against the steps before the step
// that holds it: a name or wildcard step consumes one level, another % adds
// one, and a block or path is searched from its last step. Any other step, or
// a slot that reaches the top of the expression, is the S0217 error. This file
// ports that resolution (seekParent, pushAncestry and resolveAncestry in the
// jsonata-js parser) so that the same expressions are accepted and rejected.

type ancestorSlot struct {
	level int
}

type ancestryNode struct {
	kind    string
	pos     Position
	steps   []*ancestryNode
	last    *ancestryNode
	slot    *ancestorSlot
	seeking []*ancestorSlot
	focus   bool
}

type ancestryError struct {
	kind string
	pos  Position
}

func checkAncestry(n Node) *ParseError {
	result, err := processAncestry(n)
	if err == nil && (result.kind == "parent" || len(result.seeking) > 0) {
		err = &ancestryError{kind: result.kind, pos: result.pos}
	}
	if err != nil {
		return &ParseError{Code: "S0217", Token: err.kind, Position: err.pos.Offset + 1, Message: "the parent operator cannot be resolved"}
	}
	return nil
}

func seekParent(node *ancestryNode, slot *ancestorSlot) *ancestryError {
	switch node.kind {
	case "name", "wildcard":
		slot.level--
	case "parent":
		slot.level++
	case "block":
		if node.last != nil {
			return seekParent(node.last, slot)
		}
	case "path":
		index := len(node.steps) - 1
		if err := seekParent(node.steps[index], slot); err != nil {
			return err
		}
		for index--; slot.level > 0 && index >= 0; index-- {
			if err := seekParent(node.steps[index], slot); err != nil {
				return err
			}
		}
	default:
		return &ancestryError{kind: node.kind, pos: node.pos}
	}
	return nil
}

func pushAncestry(result, value *ancestryNode) {
	result.seeking = append(result.seeking, value.seeking...)
	if value.kind == "parent" {
		result.seeking = append(result.seeking, value.slot)
	}
}

func resolveAncestry(path *ancestryNode) *ancestryError {
	last := path.steps[len(path.steps)-1]
	slots := append([]*ancestorSlot(nil), last.seeking...)
	if last.kind == "parent" {
		slots = append(slots, last.slot)
	}
	for _, slot := range slots {
		index := len(path.steps) - 2
		for slot.level > 0 {
			if index < 0 {
				path.seeking = append(path.seeking, slot)
				break
			}
			step := path.steps[index]
			index--
			for index >= 0 && step.focus && path.steps[index].focus {
				step = path.steps[index]
				index--
			}
			if err := seekParent(step, slot); err != nil {
				return err
			}
		}
	}
	return nil
}

func asPath(n *ancestryNode) *ancestryNode {
	if n.kind == "path" {
		return n
	}
	path := &ancestryNode{kind: "path", pos: n.pos, steps: []*ancestryNode{n}}
	if n.kind == "parent" {
		path.seeking = []*ancestorSlot{n.slot}
	}
	return path
}

func lastStep(n *ancestryNode) *ancestryNode {
	if n.kind == "path" {
		return n.steps[len(n.steps)-1]
	}
	return n
}

func processAncestryAll(result *ancestryNode, nodes ...Node) *ancestryError {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		value, err := processAncestry(n)
		if err != nil {
			return err
		}
		pushAncestry(result, value)
	}
	return nil
}

func discardAncestry(nodes ...Node) *ancestryError {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if _, err := processAncestry(n); err != nil {
			return err
		}
	}
	return nil
}

func processAncestry(n Node) (*ancestryNode, *ancestryError) {
	switch x := n.(type) {
	case Name:
		return &ancestryNode{kind: "path", pos: x.Pos, steps: []*ancestryNode{{kind: "name", pos: x.Pos}}}, nil
	case Wildcard:
		if x.Recursive {
			return &ancestryNode{kind: "descendant", pos: x.Pos}, nil
		}
		return &ancestryNode{kind: "wildcard", pos: x.Pos}, nil
	case Parent:
		return &ancestryNode{kind: "parent", pos: x.Pos, slot: &ancestorSlot{level: 1}}, nil
	case Variable:
		return &ancestryNode{kind: "variable", pos: x.Pos}, nil
	case Literal:
		switch x.Kind {
		case String:
			return &ancestryNode{kind: "string", pos: x.Pos}, nil
		case Number:
			return &ancestryNode{kind: "number", pos: x.Pos}, nil
		case Regex:
			return &ancestryNode{kind: "regex", pos: x.Pos}, nil
		default:
			return &ancestryNode{kind: "value", pos: x.Pos}, nil
		}
	case Path:
		result := &ancestryNode{kind: "path", pos: x.Pos}
		base, err := processAncestry(x.Base)
		if err != nil {
			return nil, err
		}
		result.steps = append(result.steps, asPath(base).steps...)
		result.seeking = append(result.seeking, asPath(base).seeking...)
		for _, field := range x.Fields {
			result.steps = append(result.steps, &ancestryNode{kind: "name", pos: field.Pos})
		}
		return result, nil
	case Block:
		result := &ancestryNode{kind: "block", pos: x.Pos}
		for _, expression := range x.Expressions {
			value, err := processAncestry(expression)
			if err != nil {
				return nil, err
			}
			pushAncestry(result, value)
			result.last = value
		}
		return result, nil
	case Array:
		result := &ancestryNode{kind: "unary", pos: x.Pos}
		return result, processAncestryAll(result, x.Items...)
	case Object:
		result := &ancestryNode{kind: "unary", pos: x.Pos}
		for _, pair := range x.Pairs {
			if err := processAncestryAll(result, pair.KeyExpr, pair.Value); err != nil {
				return nil, err
			}
		}
		return result, nil
	case Unary:
		result := &ancestryNode{kind: "unary", pos: x.Pos}
		if literal, ok := x.Expr.(Literal); ok && x.Op == "-" && literal.Kind == Number {
			return &ancestryNode{kind: "number", pos: x.Pos}, nil
		}
		return result, processAncestryAll(result, x.Expr)
	case Call:
		result := &ancestryNode{kind: "function", pos: x.Pos}
		if err := processAncestryAll(result, x.Args...); err != nil {
			return nil, err
		}
		return result, discardAncestry(x.Function)
	case Lambda:
		return &ancestryNode{kind: "lambda", pos: x.Pos}, discardAncestry(x.Body)
	case Transform:
		return &ancestryNode{kind: "transform", pos: x.Pos}, discardAncestry(x.Path, x.Update, x.Delete)
	case Apply:
		return &ancestryNode{kind: "apply", pos: x.Pos}, discardAncestry(x.Left, x.Right)
	case Bind:
		result := &ancestryNode{kind: "bind", pos: x.Pos}
		return result, processAncestryAll(result, x.Value)
	case Selector:
		return selectorAncestry(x)
	case Binary:
		return binaryAncestry(x)
	default:
		return &ancestryNode{kind: "operator"}, nil
	}
}

func selectorAncestry(x Selector) (*ancestryNode, *ancestryError) {
	result, err := processAncestry(x.Base)
	if err != nil {
		return nil, err
	}
	if x.Index == nil {
		return result, nil
	}
	step := lastStep(result)
	predicate, err := processAncestry(x.Index)
	if err != nil {
		return nil, err
	}
	for _, slot := range predicate.seeking {
		if slot.level == 1 {
			if err := seekParent(step, slot); err != nil {
				return nil, err
			}
		} else {
			slot.level--
		}
	}
	pushAncestry(step, predicate)
	return result, nil
}

func binaryAncestry(x Binary) (*ancestryNode, *ancestryError) {
	switch x.Op {
	case ".":
		return pathAncestry(x)
	case "@", "#":
		result, err := processAncestry(x.Left)
		if err != nil {
			return nil, err
		}
		if x.Op == "#" {
			result = asPath(result)
		}
		if x.Op == "@" {
			lastStep(result).focus = true
		}
		return result, nil
	case "^":
		left, err := processAncestry(x.Left)
		if err != nil {
			return nil, err
		}
		result := asPath(left)
		sortStep := &ancestryNode{kind: "sort", pos: x.Pos}
		terms := []Node{x.Right}
		if array, ok := x.Right.(Array); ok {
			terms = array.Items
		}
		for _, term := range terms {
			if unary, ok := term.(Unary); ok && (unary.Op == "<" || unary.Op == ">") {
				term = unary.Expr
			}
			if err := processAncestryAll(sortStep, term); err != nil {
				return nil, err
			}
		}
		result.steps = append(result.steps, sortStep)
		return result, resolveAncestry(result)
	case "?", "?:", "??":
		result := &ancestryNode{kind: "condition", pos: x.Pos}
		return result, processAncestryAll(result, x.Left, x.Right)
	default:
		result := &ancestryNode{kind: "binary", pos: x.Pos}
		return result, processAncestryAll(result, x.Left, x.Right)
	}
}

func pathAncestry(x Binary) (*ancestryNode, *ancestryError) {
	left, err := processAncestry(x.Left)
	if err != nil {
		return nil, err
	}
	result := asPath(left)
	if object, ok := x.Right.(Object); ok && object.Pos.Offset == x.Pos.Offset {
		for _, pair := range object.Pairs {
			if err := discardAncestry(pair.KeyExpr, pair.Value); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	right, err := processAncestry(x.Right)
	if err != nil {
		return nil, err
	}
	if nested, ok := x.Right.(Binary); ok && nested.Op == "." {
		block := &ancestryNode{kind: "block", pos: right.pos, last: right}
		pushAncestry(block, right)
		right = block
	}
	if right.kind == "path" {
		result.steps = append(result.steps, right.steps...)
	} else {
		if right.kind == "string" {
			right.kind = "name"
		}
		result.steps = append(result.steps, right)
	}
	return result, resolveAncestry(result)
}
