package btree

// LessFunc defines a function type for comparing two elements.
type LessFunc[T any] func(a, b T) bool

// ItemIteratorG defines a function type for iterating over items in the tree.
type ItemIteratorG[T any] func(item T) bool

type Item interface {
	// Less tests whether the current item is less than the given argument.
	//
	// This must provide a strict weak ordering.
	// If !a.Less(b) && !b.Less(a), we treat this to mean a == b (i.e. we can only
	// hold one of either a or b in the tree).
	Less(than Item) bool
}

type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~float32 | ~float64 | ~string
}

// BTreeG represents a B+ tree.
type BTreeG[T any] struct {
	degree   int
	less     LessFunc[T]
	root     *nodeG[T]
	length   int
	freelist *FreeListG[T]
}

// nodeG represents a node in the B+ tree.
type nodeG[T any] struct {
	items    []T
	children []*nodeG[T]
	leaf     bool
}

// FreeListG represents a freelist for reusing nodes.
type FreeListG[T any] struct {
	nodes []*nodeG[T]
}

// NewG creates a new B+ tree with the specified degree and less function.
func NewG[T any](degree int, less LessFunc[T]) *BTreeG[T] {
	return &BTreeG[T]{
		degree: degree,
		less:   less,
		root:   &nodeG[T]{leaf: true},
	}
}

// NewOrderedG creates a new B+ tree with the specified degree for ordered types.
func NewOrderedG[T Ordered](degree int) *BTreeG[T] {
	return NewG(degree, func(a, b T) bool { return a < b })
}

// NewWithFreeListG creates a new B+ tree with the specified degree, less function, and freelist.
func NewWithFreeListG[T any](degree int, less LessFunc[T], f *FreeListG[T]) *BTreeG[T] {
	return &BTreeG[T]{
		degree:   degree,
		less:     less,
		root:     &nodeG[T]{leaf: true},
		freelist: f,
	}
}

// The following methods provide various traversal and manipulation operations.

func (t *BTreeG[T]) Ascend(iterator ItemIteratorG[T]) {
	t.ascend(t.root, iterator)
}

func (t *BTreeG[T]) ascend(n *nodeG[T], iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for _, item := range n.items {
			if !iterator(item) {
				return false
			}
		}
		return true
	}
	for i, item := range n.items {
		if !t.ascend(n.children[i], iterator) {
			return false
		}
		if !iterator(item) {
			return false
		}
	}
	return t.ascend(n.children[len(n.children)-1], iterator)
}

func (t *BTreeG[T]) AscendGreaterOrEqual(pivot T, iterator ItemIteratorG[T]) {
	t.ascendGreaterOrEqual(t.root, pivot, iterator)
}

func (t *BTreeG[T]) ascendGreaterOrEqual(n *nodeG[T], pivot T, iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for _, item := range n.items {
			if !t.less(item, pivot) && !iterator(item) {
				return false
			}
		}
		return true
	}
	for i, item := range n.items {
		if t.less(pivot, item) || !t.less(item, pivot) {
			if !t.ascendGreaterOrEqual(n.children[i], pivot, iterator) {
				return false
			}
			if !iterator(item) {
				return false
			}
		}
	}
	return t.ascendGreaterOrEqual(n.children[len(n.children)-1], pivot, iterator)
}

func (t *BTreeG[T]) AscendLessThan(pivot T, iterator ItemIteratorG[T]) {
	t.ascendLessThan(t.root, pivot, iterator)
}

func (t *BTreeG[T]) ascendLessThan(n *nodeG[T], pivot T, iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for _, item := range n.items {
			if t.less(item, pivot) && !iterator(item) {
				return false
			}
		}
		return true
	}
	for i, item := range n.items {
		if t.less(item, pivot) {
			if !t.ascendLessThan(n.children[i], pivot, iterator) {
				return false
			}
			if !iterator(item) {
				return false
			}
		} else {
			return t.ascendLessThan(n.children[i], pivot, iterator)
		}
	}
	return t.ascendLessThan(n.children[len(n.children)-1], pivot, iterator)
}

func (t *BTreeG[T]) AscendRange(greaterOrEqual, lessThan T, iterator ItemIteratorG[T]) {
	t.ascendRange(t.root, greaterOrEqual, lessThan, iterator)
}

func (t *BTreeG[T]) ascendRange(n *nodeG[T], greaterOrEqual, lessThan T, iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for _, item := range n.items {
			if !t.less(item, greaterOrEqual) && t.less(item, lessThan) && !iterator(item) {
				return false
			}
		}
		return true
	}
	for i, item := range n.items {
		if t.less(greaterOrEqual, item) || !t.less(item, greaterOrEqual) {
			if !t.ascendRange(n.children[i], greaterOrEqual, lessThan, iterator) {
				return false
			}
			if !t.less(item, lessThan) && !iterator(item) {
				return false
			}
		}
	}
	return t.ascendRange(n.children[len(n.children)-1], greaterOrEqual, lessThan, iterator)
}

func (t *BTreeG[T]) Clear(addNodesToFreelist bool) {
	if addNodesToFreelist {
		t.addNodesToFreelist(t.root)
	}
	t.root = &nodeG[T]{leaf: true}
	t.length = 0
}

func (t *BTreeG[T]) addNodesToFreelist(n *nodeG[T]) {
	if !n.leaf {
		for _, child := range n.children {
			t.addNodesToFreelist(child)
		}
	}
	t.freelist.nodes = append(t.freelist.nodes, n)
}

func (t *BTreeG[T]) Clone() *BTreeG[T] {
	clone := *t
	clone.root = t.cloneNode(t.root)
	return &clone
}

func (t *BTreeG[T]) cloneNode(n *nodeG[T]) *nodeG[T] {
	clone := *n
	if !n.leaf {
		clone.children = make([]*nodeG[T], len(n.children))
		for i, child := range n.children {
			clone.children[i] = t.cloneNode(child)
		}
	}
	return &clone
}

func (t *BTreeG[T]) Delete(item T) (T, bool) {
	if t.root == nil {
		var zero T
		return zero, false
	}
	var deleted T
	t.root, deleted = t.delete(t.root, item)
	if len(t.root.items) == 0 && !t.root.leaf {
		t.root = t.root.children[0]
	}
	if len(t.root.items) == 0 {
		t.root = nil
	}
	if !t.root.leaf {
		t.length--
	}
	return deleted, true
}

func (t *BTreeG[T]) delete(n *nodeG[T], item T) (*nodeG[T], T) {
	if n == nil {
		var zero T
		return nil, zero
	}

	var deleted T

	for i, current := range n.items {
		if !t.less(current, item) && !t.less(item, current) {
			// アイテムを見つけた場合
			deleted = current

			if n.leaf {
				// 葉ノードの場合、アイテムを直接削除
				n.items = append(n.items[:i], n.items[i+1:]...)
				return n, deleted
			}

			// 内部ノードの場合、右サブツリーの最小アイテムで置き換え
			n.items[i] = t.getMin(n.children[i+1])
			n.children[i+1], _ = t.deleteMin(n.children[i+1])
			return n, deleted
		} else if t.less(item, current) {
			// アイテムが現在のアイテムより小さい場合、左の子に移動
			n.children[i], deleted = t.delete(n.children[i], item)
			if len(n.children[i].items) == 0 {
				n.children = append(n.children[:i], n.children[i+1:]...)
			}
			return n, deleted
		}
	}

	// アイテムが見つからなかった場合、最後の子に移動
	n.children[len(n.children)-1], deleted = t.delete(n.children[len(n.children)-1], item)
	if len(n.children[len(n.children)-1].items) == 0 {
		n.children = n.children[:len(n.children)-1]
	}

	return n, deleted
}

func (t *BTreeG[T]) getMin(n *nodeG[T]) T {
	for !n.leaf {
		n = n.children[0]
	}
	return n.items[0]
}

// DeleteMax and DeleteMin are similar methods, adapted for max/min.

func (t *BTreeG[T]) DeleteMax() (T, bool) {
	if t.root == nil {
		var zero T
		return zero, false
	}
	var max T
	t.root, max = t.deleteMax(t.root)
	if len(t.root.items) == 0 && !t.root.leaf {
		t.root = t.root.children[0]
	}
	if len(t.root.items) == 0 {
		t.root = nil
	}
	if !t.root.leaf {
		t.length--
	}
	return max, true
}

func (t *BTreeG[T]) deleteMax(n *nodeG[T]) (*nodeG[T], T) {
	if n.leaf {
		max := n.items[len(n.items)-1]
		n.items = n.items[:len(n.items)-1]
		return n, max
	}
	lastChild := n.children[len(n.children)-1]
	var max T
	n.children[len(n.children)-1], max = t.deleteMax(lastChild)
	if len(lastChild.items) == 0 {
		n.children = n.children[:len(n.children)-1]
	}
	return n, max
}

func (t *BTreeG[T]) DeleteMin() (T, bool) {
	if t.root == nil {
		var zero T
		return zero, false
	}
	var min T
	t.root, min = t.deleteMin(t.root)
	if len(t.root.items) == 0 && !t.root.leaf {
		t.root = t.root.children[0]
	}
	if len(t.root.items) == 0 {
		t.root = nil
	}
	if !t.root.leaf {
		t.length--
	}
	return min, true
}

func (t *BTreeG[T]) deleteMin(n *nodeG[T]) (*nodeG[T], T) {
	if n.leaf {
		min := n.items[0]
		n.items = n.items[1:]
		return n, min
	}
	firstChild := n.children[0]
	var min T
	n.children[0], min = t.deleteMin(firstChild)
	if len(firstChild.items) == 0 {
		n.children = n.children[1:]
	}
	return n, min
}

func (t *BTreeG[T]) Descend(iterator ItemIteratorG[T]) {
	t.descend(t.root, iterator)
}

func (t *BTreeG[T]) descend(n *nodeG[T], iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for i := len(n.items) - 1; i >= 0; i-- {
			if !iterator(n.items[i]) {
				return false
			}
		}
		return true
	}
	for i := len(n.items) - 1; i >= 0; i-- {
		if !t.descend(n.children[i+1], iterator) {
			return false
		}
		if !iterator(n.items[i]) {
			return false
		}
	}
	return t.descend(n.children[0], iterator)
}

func (t *BTreeG[T]) DescendGreaterThan(pivot T, iterator ItemIteratorG[T]) {
	t.descendGreaterThan(t.root, pivot, iterator)
}

func (t *BTreeG[T]) descendGreaterThan(n *nodeG[T], pivot T, iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for i := len(n.items) - 1; i >= 0; i-- {
			if t.less(pivot, n.items[i]) && !iterator(n.items[i]) {
				return false
			}
		}
		return true
	}
	for i := len(n.items) - 1; i >= 0; i-- {
		if t.less(pivot, n.items[i]) {
			if !t.descendGreaterThan(n.children[i+1], pivot, iterator) {
				return false
			}
			if !iterator(n.items[i]) {
				return false
			}
		} else {
			return t.descendGreaterThan(n.children[i+1], pivot, iterator)
		}
	}
	return t.descendGreaterThan(n.children[0], pivot, iterator)
}

func (t *BTreeG[T]) DescendLessOrEqual(pivot T, iterator ItemIteratorG[T]) {
	t.descendLessOrEqual(t.root, pivot, iterator)
}

func (t *BTreeG[T]) descendLessOrEqual(n *nodeG[T], pivot T, iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for i := len(n.items) - 1; i >= 0; i-- {
			if !t.less(n.items[i], pivot) && !iterator(n.items[i]) {
				return false
			}
		}
		return true
	}
	for i := len(n.items) - 1; i >= 0; i-- {
		if !t.less(n.items[i], pivot) {
			if !t.descendLessOrEqual(n.children[i+1], pivot, iterator) {
				return false
			}
			if !iterator(n.items[i]) {
				return false
			}
		}
	}
	return t.descendLessOrEqual(n.children[0], pivot, iterator)
}

func (t *BTreeG[T]) DescendRange(lessOrEqual, greaterThan T, iterator ItemIteratorG[T]) {
	t.descendRange(t.root, lessOrEqual, greaterThan, iterator)
}

func (t *BTreeG[T]) descendRange(n *nodeG[T], lessOrEqual, greaterThan T, iterator ItemIteratorG[T]) bool {
	if n.leaf {
		for i := len(n.items) - 1; i >= 0; i-- {
			if !t.less(lessOrEqual, n.items[i]) && t.less(greaterThan, n.items[i]) && !iterator(n.items[i]) {
				return false
			}
		}
		return true
	}
	for i := len(n.items) - 1; i >= 0; i-- {
		if !t.less(lessOrEqual, n.items[i]) && t.less(greaterThan, n.items[i]) {
			if !t.descendRange(n.children[i+1], lessOrEqual, greaterThan, iterator) {
				return false
			}
			if !iterator(n.items[i]) {
				return false
			}
		}
	}
	return t.descendRange(n.children[0], lessOrEqual, greaterThan, iterator)
}

func (t *BTreeG[T]) Get(key T) (T, bool) {
	return t.get(t.root, key)
}

func (t *BTreeG[T]) get(n *nodeG[T], key T) (T, bool) {
	if n == nil {
		var zero T
		return zero, false
	}
	for i, item := range n.items {
		if !t.less(item, key) && !t.less(key, item) {
			return item, true
		} else if t.less(key, item) {
			return t.get(n.children[i], key)
		}
	}
	return t.get(n.children[len(n.children)-1], key)
}

func (t *BTreeG[T]) Has(key T) bool {
	_, found := t.Get(key)
	return found
}

func (t *BTreeG[T]) Len() int {
	return t.length
}

func (t *BTreeG[T]) Max() (T, bool) {
	if t.root == nil {
		var zero T
		return zero, false
	}
	n := t.root
	for !n.leaf {
		n = n.children[len(n.children)-1]
	}
	return n.items[len(n.items)-1], true
}

func (t *BTreeG[T]) Min() (T, bool) {
	if t.root == nil {
		var zero T
		return zero, false
	}
	n := t.root
	for !n.leaf {
		n = n.children[0]
	}
	return n.items[0], true
}

func (t *BTreeG[T]) ReplaceOrInsert(item T) (T, bool) {
	if t.root == nil {
		t.root = &nodeG[T]{items: []T{item}, leaf: true}
		t.length++
		return item, false
	}
	var replaced T
	var found bool
	t.root, replaced, found = t.insert(t.root, item)
	if len(t.root.items) >= t.degree {
		newRoot := &nodeG[T]{leaf: false}
		newRoot.children = []*nodeG[T]{t.root}
		t.split(newRoot, 0)
		t.root = newRoot
	}
	if !found {
		t.length++
	}
	return replaced, found
}

func (t *BTreeG[T]) insert(n *nodeG[T], item T) (*nodeG[T], T, bool) {
	if n.leaf {
		for i, existing := range n.items {
			if !t.less(existing, item) && !t.less(item, existing) {
				n.items[i] = item
				return n, existing, true
			}
			if t.less(item, existing) {
				n.items = append(n.items[:i], append([]T{item}, n.items[i:]...)...)
				return n, item, false
			}
		}
		n.items = append(n.items, item)
		return n, item, false
	}
	for i, existing := range n.items {
		if t.less(item, existing) {
			var replaced T
			var found bool
			n.children[i], replaced, found = t.insert(n.children[i], item)
			if len(n.children[i].items) >= t.degree {
				t.split(n, i)
			}
			return n, replaced, found
		}
	}
	var replaced T
	var found bool
	n.children[len(n.children)-1], replaced, found = t.insert(n.children[len(n.children)-1], item)
	if len(n.children[len(n.children)-1].items) >= t.degree {
		t.split(n, len(n.children)-1)
	}
	return n, replaced, found
}

func (t *BTreeG[T]) split(parent *nodeG[T], index int) {
	fullChild := parent.children[index]
	splitIndex := t.degree / 2
	newChild := &nodeG[T]{
		items:    append([]T{}, fullChild.items[splitIndex+1:]...),
		leaf:     fullChild.leaf,
		children: append([]*nodeG[T]{}, fullChild.children[splitIndex+1:]...),
	}
	parent.items = append(parent.items[:index], append([]T{fullChild.items[splitIndex]}, parent.items[index:]...)...)
	parent.children = append(parent.children[:index+1], append([]*nodeG[T]{newChild}, parent.children[index+1:]...)...)
	fullChild.items = fullChild.items[:splitIndex]
	fullChild.children = fullChild.children[:splitIndex+1]
}
