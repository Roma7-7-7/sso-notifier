// Here I'll store non production code that is necessary for manual actions.
// In most cases those will be one time actions
package main

// func notifyAll() {
//	store := NewBoltDBStore("data/app.db")
//	subs, err := store.GetSubscribers()
//	if err != nil {
//		panic(err)
//	}
//
//	for _, s := range subs {
//		if _, err = store.QueueNotification(s, `У зв'язку зі зміннами на сайті Чернівціобленерго бот тимчасово не працює =(
// Пробую вирішити проблему`); err != nil {
//			panic(err)
//		}
//	}
//
//	zap.L().Info("Done")
// }
